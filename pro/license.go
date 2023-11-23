//go:build ee
// +build ee

package pro

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

const (
	db_license_key = "netmaker-id-key-pair"
)

type apiServerConf struct {
	PrivateKey []byte `json:"private_key" binding:"required"`
	PublicKey  []byte `json:"public_key"  binding:"required"`
}

// AddLicenseHooks - adds the validation and cache clear hooks
func AddLicenseHooks() {
	logic.HookManagerCh <- models.HookDetails{
		Hook:     ValidateLicense,
		Interval: time.Hour,
	}
	logic.HookManagerCh <- models.HookDetails{
		Hook:     ClearLicenseCache,
		Interval: time.Hour,
	}
}

// ValidateLicense - the initial and periodic license check for netmaker server
// checks if a license is valid + limits are not exceeded
// if license is free_tier and limits exceeds, then function should error
// if license is not valid, function should error
func ValidateLicense() (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %s", errValidation, err.Error())
		}
		servercfg.ErrLicenseValidation = err
	}()

	licenseKeyValue := servercfg.GetLicenseKey()
	netmakerTenantID := servercfg.GetNetmakerTenantID()
	slog.Info("proceeding with Netmaker license validation...")
	if len(licenseKeyValue) == 0 {
		err = errors.New("empty license-key (LICENSE_KEY environment variable)")
		return err
	}
	if len(netmakerTenantID) == 0 {
		err = errors.New("empty tenant-id (NETMAKER_TENANT_ID environment variable)")
		return err
	}

	apiPublicKey, err := getLicensePublicKey(licenseKeyValue)
	if err != nil {
		err = fmt.Errorf("failed to get license public key: %w", err)
		return err
	}

	tempPubKey, tempPrivKey, err := FetchApiServerKeys()
	if err != nil {
		err = fmt.Errorf("failed to fetch api server keys: %w", err)
		return err
	}

	licenseSecret := LicenseSecret{
		AssociatedID: netmakerTenantID,
		Usage:        getCurrentServerUsage(),
	}

	secretData, err := json.Marshal(&licenseSecret)
	if err != nil {
		err = fmt.Errorf("failed to marshal license secret: %w", err)
		return err
	}

	encryptedData, err := ncutils.BoxEncrypt(secretData, apiPublicKey, tempPrivKey)
	if err != nil {
		err = fmt.Errorf("failed to encrypt license secret data: %w", err)
		return err
	}

	validationResponse, timedOut, err := validateLicenseKey(encryptedData, tempPubKey)
	if err != nil {
		err = fmt.Errorf("failed to validate license key: %w", err)
		return err
	}
	if timedOut {
		return
	}
	if len(validationResponse) == 0 {
		err = errors.New("empty validation response")
		return err
	}

	var licenseResponse ValidatedLicense
	if err = json.Unmarshal(validationResponse, &licenseResponse); err != nil {
		err = fmt.Errorf("failed to unmarshal validation response: %w", err)
		return err
	}

	respData, err := ncutils.BoxDecrypt(
		base64decode(licenseResponse.EncryptedLicense),
		apiPublicKey,
		tempPrivKey,
	)
	if err != nil {
		err = fmt.Errorf("failed to decrypt license: %w", err)
		return err
	}

	license := LicenseKey{}
	if err = json.Unmarshal(respData, &license); err != nil {
		err = fmt.Errorf("failed to unmarshal license key: %w", err)
		return err
	}

	slog.Info("License validation succeeded!")
	return nil
}

// FetchApiServerKeys - fetches netmaker license keys for identification
// as well as secure communication with API
// if none present, it generates a new pair
func FetchApiServerKeys() (pub *[32]byte, priv *[32]byte, err error) {
	returnData := apiServerConf{}
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

func getLicensePublicKey(licensePubKeyEncoded string) (*[32]byte, error) {
	decodedPubKey := base64decode(licensePubKeyEncoded)
	return ncutils.ConvertBytesToKey(decodedPubKey)
}

func validateLicenseKey(encryptedData []byte, publicKey *[32]byte) ([]byte, bool, error) {
	publicKeyBytes, err := ncutils.ConvertKeyToBytes(publicKey)
	if err != nil {
		return nil, false, err
	}
	msg := ValidateLicenseRequest{
		LicenseKey:     servercfg.GetLicenseKey(),
		NmServerPubKey: base64encode(publicKeyBytes),
		EncryptedPart:  base64encode(encryptedData),
	}

	requestBody, err := json.Marshal(msg)
	if err != nil {
		return nil, false, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		getAccountsHost()+"/api/v1/license/validate",
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return nil, false, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	client := &http.Client{}
	validateResponse, err := client.Do(req)
	if err != nil { // check cache
		slog.Warn("proceeding with cached response, Netmaker API may be down")
		cachedResp, err := getCachedResponse()
		return cachedResp, false, err
	}
	defer validateResponse.Body.Close()
	code := validateResponse.StatusCode

	// if we received a 200, cache the response locally
	if code == http.StatusOK {
		body, err := io.ReadAll(validateResponse.Body)
		if err != nil {
			slog.Warn("failed to parse response", "error", err)
			return nil, false, err
		}
		if err := cacheResponse(body); err != nil {
			slog.Warn("failed to cache response", "error", err)
		}
		return body, false, nil
	}

	// at this point the backend returned some undesired state

	// inform failure via logs
	body, _ := io.ReadAll(validateResponse.Body)
	err = fmt.Errorf("could not validate license with validation backend (status={%d}, body={%s})",
		validateResponse.StatusCode, string(body))
	slog.Warn(err.Error())

	// try to use cache if we had a temporary error
	if code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout {
		slog.Warn("Netmaker API may be down, will retry later...", "code", code)
		return nil, true, nil
	}

	// at this point the error is irreversible, return it
	return nil, false, err
}

func getAccountsHost() string {
	switch servercfg.GetEnvironment() {
	case "dev":
		return accountsHostDevelopment
	case "staging":
		return accountsHostStaging
	default:
		return accountsHostProduction
	}
}

func cacheResponse(response []byte) error {
	lrc := licenseResponseCache{
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
