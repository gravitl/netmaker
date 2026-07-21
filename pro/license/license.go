package license

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/mq"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/utils"
	"gorm.io/gorm"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

// AddLicenseHooks - adds the validation and cache clear hooks
func AddLicenseHooks() {
	logic.HookManagerCh <- models.HookDetails{
		ID: "license-validation-hook",
		Hook: logic.WrapHook(func() error {
			return ValidateLicense()
		}),
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
		Usage:        logic.GetCurrentServerUsage(db.WithContext(context.TODO())),
		TenantUsage:  make(map[string]models.Usage),
	}

	tenants, err := (&schema.Tenant{}).List(db.WithContext(context.TODO()))
	if err != nil {
		err = fmt.Errorf("failed to list tenants: %w", err)
		return err
	}

	for _, tenant := range tenants {
		ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, tenant.ID)
		licenseSecret.TenantUsage[tenant.ID] = logic.GetCurrentServerUsage(ctx)
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

	proLogic.SetFeatureFlags(licenseResponse.FeatureFlags)
	proLogic.SetDeploymentMode(licenseResponse.DeploymentMode)

	go mq.PublishExporterFeatureFlags()
	go func() {
		ctx := db.WithContext(context.Background())
		tenants, err := (&schema.Tenant{}).List(ctx)
		if err != nil {
			slog.Error("failed to list tenants for peer update", "error", err)
			return
		}
		for _, tenant := range tenants {
			ctx := scope.WithContext(ctx, scope.TenantScope, tenant.ID)
			mq.PublishPeerUpdate(ctx, false)
		}
	}()

	slog.Info("License validation succeeded!")
	return nil
}

// FetchApiServerKeys - fetches netmaker license keys for identification
// as well as secure communication with API
// if none present, it generates a new pair
func FetchApiServerKeys(ctx context.Context) (pub *[32]byte, priv *[32]byte, err error) {
	var create bool
	privateKey := &schema.Internal{
		Key: schema.InternalKey_LicenseValidationPrivateKey,
	}
	err = privateKey.Get(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			create = true
		} else {
			return nil, nil, err
		}
	}

	publicKey := &schema.Internal{
		Key: schema.InternalKey_LicenseValidationPublicKey,
	}
	err = publicKey.Get(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			create = true
		} else {
			return nil, nil, err
		}
	}

	if create {
		pub, priv, err = box.GenerateKey(rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		privateKeyBytes, err := ncutils.ConvertKeyToBytes(priv)
		if err != nil {
			return nil, nil, err
		}
		publicKeyBytes, err := ncutils.ConvertKeyToBytes(pub)
		if err != nil {
			return nil, nil, err
		}

		privateKey.Value = base64encode(privateKeyBytes)
		publicKey.Value = base64encode(publicKeyBytes)
		err = privateKey.Set(ctx)
		if err != nil {
			return nil, nil, err
		}

		err = publicKey.Set(ctx)
		if err != nil {
			return nil, nil, err
		}
	} else {
		priv, err = ncutils.ConvertBytesToKey(base64decode(privateKey.Value))
		if err != nil {
			return nil, nil, err
		}
		pub, err = ncutils.ConvertBytesToKey(base64decode(publicKey.Value))
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

func validateLicenseKey(ctx context.Context, encryptedData []byte, publicKey *[32]byte) ([]byte, bool, error) {
	publicKeyBytes, err := ncutils.ConvertKeyToBytes(publicKey)
	if err != nil {
		return nil, false, err
	}
	msg := ValidateLicenseRequest{
		ServerVersion:  servercfg.GetVersion(),
		LicenseKey:     servercfg.GetLicenseKey(),
		NmServerPubKey: base64encode(publicKeyBytes),
		EncryptedPart:  base64encode(encryptedData),
		NmBaseDomain:   servercfg.GetNmBaseDomain(),
	}

	requestBody, err := json.Marshal(msg)
	if err != nil {
		return nil, false, err
	}

	var validateResponse *http.Response
	var validationResponse []byte
	var timedOut bool

	validationRetries := utils.RetryStrategy{
		WaitTime:         time.Second * 5,
		WaitTimeIncrease: time.Second * 2,
		MaxTries:         15,
		Wait: func(duration time.Duration) {
			time.Sleep(duration)
		},
		Try: func() error {
			req, err := http.NewRequest(
				http.MethodPost,
				proLogic.GetAccountsHost()+"/api/v1/license/validate",
				bytes.NewReader(requestBody),
			)
			if err != nil {
				return err
			}
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			client := &http.Client{}

			validateResponse, err = client.Do(req)
			if err != nil {
				slog.Warn(fmt.Sprintf("error while validating license key: %v", err))
				return err
			}

			if validateResponse.StatusCode == http.StatusServiceUnavailable ||
				validateResponse.StatusCode == http.StatusGatewayTimeout ||
				validateResponse.StatusCode == http.StatusBadGateway {
				timedOut = true
				return errors.New("failed to reach netmaker api")
			}

			return nil
		},
		OnMaxTries: func() {
			slog.Warn("proceeding with cached response, Netmaker API may be down")
			validationResponse, err = getCachedResponse(ctx)
			timedOut = false
		},
		OnSuccess: func() {
			defer validateResponse.Body.Close()

			// if we received a 200, cache the response locally
			if validateResponse.StatusCode == http.StatusOK {
				validationResponse, err = io.ReadAll(validateResponse.Body)
				if err != nil {
					slog.Warn("failed to parse response", "error", err)
					validationResponse = nil
					timedOut = false
					return
				}

				if err := cacheResponse(ctx, validationResponse); err != nil {
					slog.Warn("failed to cache response", "error", err)
				}
			} else {
				// at this point the backend returned some undesired state

				// inform failure via logs
				body, _ := io.ReadAll(validateResponse.Body)
				err = fmt.Errorf("could not validate license with validation backend (status={%d}, body={%s})",
					validateResponse.StatusCode, string(body))
				slog.Warn(err.Error())
			}
		},
	}

	validationRetries.DoStrategy()

	return validationResponse, timedOut, err
}

func cacheResponse(ctx context.Context, response []byte) error {
	cached := &schema.Internal{
		Key:   schema.InternalKey_LicenseValidationCachedResponse,
		Value: base64encode(response),
	}
	return cached.Set(ctx)
}

func getCachedResponse(ctx context.Context) ([]byte, error) {
	cached := &schema.Internal{
		Key: schema.InternalKey_LicenseValidationCachedResponse,
	}
	err := cached.Get(ctx)
	if err != nil {
		return nil, err
	}

	return base64decode(cached.Value), nil
}
