//go:build ee
// +build ee

package ee

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/exp/slog"
	"io"
	"net/http"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/nacl/box"
)

const (
	db_license_key = "netmaker-id-key-pair"
)

type apiServerConf struct {
	PrivateKey []byte `json:"private_key" binding:"required"`
	PublicKey  []byte `json:"public_key" binding:"required"`
}

// AddLicenseHooks - adds the validation and cache clear hooks
func AddLicenseHooks() {
	logic.HookManagerCh <- models.HookDetails{
		Hook: func() error {
			if err := ValidateLicense(); err != nil {
				// stop the program when license is not valid anymore
				// if the server restarts and still fails the license check, it can reboot in a limited mode
				return fmt.Errorf("%w: %s", logic.HookManagerFatalError, err.Error())
			}
			return nil
		},
		Interval: time.Hour,
	}
	logic.HookManagerCh <- models.HookDetails{
		Hook:     ClearLicenseCache,
		Interval: time.Hour,
	}
}

// ValidateLicense - the initial license check for netmaker server
// checks if a license is valid + limits are not exceeded
// if license is free_tier and limits exceeds, then server should terminate
// if license is not valid, server should terminate
// TODO update comment
func ValidateLicense() error {
	licenseKeyValue := servercfg.GetLicenseKey()
	netmakerTenantID := servercfg.GetNetmakerTenantID()
	slog.Info("proceeding with Netmaker license validation...")
	if len(licenseKeyValue) == 0 {
		return wrappedInErrValidation(errors.New("empty license-key (LICENSE_KEY environment variable)"))
	}
	if len(netmakerTenantID) == 0 {
		return wrappedInErrValidation(errors.New("empty tenant-id (NETMAKER_TENANT_ID environment variable)"))
	}

	apiPublicKey, err := getLicensePublicKey(licenseKeyValue)
	if err != nil {
		return wrappedInErrValidation(fmt.Errorf("failed to get license public key: %w", err))
	}

	tempPubKey, tempPrivKey, err := FetchApiServerKeys()
	if err != nil {
		return wrappedInErrValidation(fmt.Errorf("failed to fetch api server keys: %w", err))
	}

	licenseSecret := LicenseSecret{
		AssociatedID: netmakerTenantID,
		Limits:       getCurrentServerLimit(),
	}

	secretData, err := json.Marshal(&licenseSecret)
	if err != nil {
		return wrappedInErrValidation(fmt.Errorf("failed to marshal license secret: %w", err))
	}

	encryptedData, err := ncutils.BoxEncrypt(secretData, apiPublicKey, tempPrivKey)
	if err != nil {
		return wrappedInErrValidation(fmt.Errorf("failed to encrypt license secret data: %w", err))
	}

	validationResponse, err := validateLicenseKey(encryptedData, tempPubKey)
	if err != nil {
		return wrappedInErrValidation(fmt.Errorf("failed to validate license key: %w", err))
	}
	if len(validationResponse) == 0 {
		return wrappedInErrValidation(errors.New("empty validation response"))
	}

	var licenseResponse ValidatedLicense
	if err = json.Unmarshal(validationResponse, &licenseResponse); err != nil {
		return wrappedInErrValidation(fmt.Errorf("failed to unmarshal validation response: %w", err))
	}

	respData, err := ncutils.BoxDecrypt(base64decode(licenseResponse.EncryptedLicense), apiPublicKey, tempPrivKey)
	if err != nil {
		return wrappedInErrValidation(fmt.Errorf("failed to decrypt license: %w", err))
	}

	license := LicenseKey{}
	if err = json.Unmarshal(respData, &license); err != nil {
		return wrappedInErrValidation(fmt.Errorf("failed to unmarshal license key: %w", err))
	}

	slog.Info("License validation succeeded!")
	return nil
}

// FetchApiServerKeys - fetches netmaker license keys for identification
// as well as secure communication with API
// if none present, it generates a new pair
func FetchApiServerKeys() (pub *[32]byte, priv *[32]byte, err error) {
	var returnData = apiServerConf{}
	currentData, err := database.FetchRecord(database.SERVERCONF_TABLE_NAME, db_license_key)
	if err != nil && !database.IsEmptyRecord(err) {
		return nil, nil, err
	} else if database.IsEmptyRecord(err) { // need to generate a new identifier pair
		pub, priv, err = box.GenerateKey(rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		pubBytes, err := ncutils.ConvertKeyToBytes(pub)
		if err != nil {
			return nil, nil, err
		}
		privBytes, err := ncutils.ConvertKeyToBytes(priv)
		if err != nil {
			return nil, nil, err
		}
		returnData.PrivateKey = privBytes
		returnData.PublicKey = pubBytes
		record, err := json.Marshal(&returnData)
		if err != nil {
			return nil, nil, err
		}
		if err = database.Insert(db_license_key, string(record), database.SERVERCONF_TABLE_NAME); err != nil {
			return nil, nil, err
		}
	} else {
		if err = json.Unmarshal([]byte(currentData), &returnData); err != nil {
			return nil, nil, err
		}
		priv, err = ncutils.ConvertBytesToKey(returnData.PrivateKey)
		if err != nil {
			return nil, nil, err
		}
		pub, err = ncutils.ConvertBytesToKey(returnData.PublicKey)
		if err != nil {
			return nil, nil, err
		}
	}

	return pub, priv, nil
}

func wrappedInErrValidation(err error) error {
	return fmt.Errorf("%w: %s", ErrValidation, err.Error())
}

func getLicensePublicKey(licensePubKeyEncoded string) (*[32]byte, error) {
	decodedPubKey := base64decode(licensePubKeyEncoded)
	return ncutils.ConvertBytesToKey(decodedPubKey)
}

func validateLicenseKey(encryptedData []byte, publicKey *[32]byte) ([]byte, error) {

	publicKeyBytes, err := ncutils.ConvertKeyToBytes(publicKey)
	if err != nil {
		return nil, err
	}

	msg := ValidateLicenseRequest{
		LicenseKey:     servercfg.GetLicenseKey(),
		NmServerPubKey: base64encode(publicKeyBytes),
		EncryptedPart:  base64encode(encryptedData),
	}

	requestBody, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, api_endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	client := &http.Client{}
	var body []byte
	validateResponse, err := client.Do(req)
	if err != nil { // check cache
		body, err = getCachedResponse()
		if err != nil {
			return nil, err
		}
		slog.Warn("proceeding with cached response, Netmaker API may be down")
	} else {
		defer validateResponse.Body.Close()
		if validateResponse.StatusCode != 200 {
			return nil, fmt.Errorf("could not validate license, got status code %d", validateResponse.StatusCode)
		} // if you received a 200 cache the response locally

		body, err = io.ReadAll(validateResponse.Body)
		if err != nil {
			return nil, err
		}
		cacheResponse(body)
	}

	return body, err
}

func cacheResponse(response []byte) error {
	var lrc = licenseResponseCache{
		Body: response,
	}

	record, err := json.Marshal(&lrc)
	if err != nil {
		return err
	}

	return database.Insert(license_cache_key, string(record), database.CACHE_TABLE_NAME)
}

func getCachedResponse() ([]byte, error) {
	var lrc licenseResponseCache
	record, err := database.FetchRecord(database.CACHE_TABLE_NAME, license_cache_key)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(record), &lrc); err != nil {
		return nil, err
	}
	return lrc.Body, nil
}

// ClearLicenseCache - clears the cached validate response
func ClearLicenseCache() error {
	return database.DeleteRecord(database.CACHE_TABLE_NAME, license_cache_key)
}
