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
	"sync/atomic"
	"time"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/mq"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/hashicorp/go-retryablehttp"
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

func fetchValidatedLicense(ctx context.Context) (licenseResponse ValidatedLicense, isCachedResp bool, err error) {
	licenseKeyValue := servercfg.GetLicenseKey()
	netmakerTenantID := servercfg.GetNetmakerTenantID()
	slog.Info("proceeding with Netmaker license validation...")
	if len(licenseKeyValue) == 0 {
		return licenseResponse, false, errors.New("empty license-key (LICENSE_KEY environment variable)")
	}
	if len(netmakerTenantID) == 0 {
		return licenseResponse, false, errors.New("empty tenant-id (NETMAKER_TENANT_ID environment variable)")
	}

	apiPublicKey, err := getLicensePublicKey(licenseKeyValue)
	if err != nil {
		return licenseResponse, false, fmt.Errorf("failed to get license public key: %w", err)
	}

	tempPubKey, tempPrivKey, err := FetchApiServerKeys(ctx)
	if err != nil {
		return licenseResponse, false, fmt.Errorf("failed to fetch api server keys: %w", err)
	}

	licenseSecret := LicenseSecret{
		AssociatedID: netmakerTenantID,
		Usage:        logic.GetCurrentServerUsage(ctx),
		TenantUsage:  make(map[string]models.Usage),
	}

	tenants, err := (&schema.Tenant{}).List(ctx)
	if err != nil {
		return licenseResponse, false, fmt.Errorf("failed to list tenants: %w", err)
	}

	for _, tenant := range tenants {
		tenantCtx := scope.WithContext(ctx, scope.TenantScope, tenant.ID)
		licenseSecret.TenantUsage[tenant.ID] = logic.GetCurrentServerUsage(tenantCtx)
	}

	secretData, err := json.Marshal(&licenseSecret)
	if err != nil {
		return licenseResponse, false, fmt.Errorf("failed to marshal license secret: %w", err)
	}

	encryptedData, err := ncutils.BoxEncrypt(secretData, apiPublicKey, tempPrivKey)
	if err != nil {
		return licenseResponse, false, fmt.Errorf("failed to encrypt license secret data: %w", err)
	}

	validationResponse, apiErr := callLicenseValidationApi(ctx, encryptedData, tempPubKey)
	if apiErr != nil {
		slog.Warn("failed to validate license key, falling back to cached response", "error", apiErr)
		validationResponse, err = getCachedResponseFromDB(ctx)
		if err != nil {
			return licenseResponse, false, fmt.Errorf("failed to validate license key: %w", apiErr)
		}
		isCachedResp = true
	} else {
		if err := cacheResponseInDB(ctx, validationResponse); err != nil {
			slog.Warn("failed to cache response", "error", err)
		}
	}

	if len(validationResponse) == 0 {
		return licenseResponse, false, errors.New("empty validation response")
	}

	if err = json.Unmarshal(validationResponse, &licenseResponse); err != nil {
		return licenseResponse, false, fmt.Errorf("failed to unmarshal validation response: %w", err)
	}

	respData, err := ncutils.BoxDecrypt(
		base64decode(licenseResponse.EncryptedLicense),
		apiPublicKey,
		tempPrivKey,
	)
	if err != nil {
		return licenseResponse, false, fmt.Errorf("failed to decrypt license: %w", err)
	}

	license := LicenseKey{}
	if err = json.Unmarshal(respData, &license); err != nil {
		return licenseResponse, false, fmt.Errorf("failed to unmarshal license key: %w", err)
	}

	if !licenseResponse.Expiry.IsZero() && time.Now().After(licenseResponse.Expiry) {
		return licenseResponse, isCachedResp, fmt.Errorf("license validation response expired at %s", licenseResponse.Expiry)
	}

	return licenseResponse, isCachedResp, nil
}

func callLicenseValidationApi(ctx context.Context, encryptedData []byte, publicKey *[32]byte) ([]byte, error) {
	publicKeyBytes, err := ncutils.ConvertKeyToBytes(publicKey)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	client := retryablehttp.NewClient()
	client.Logger = nil
	client.RetryMax = 15
	client.RetryWaitMin = time.Second * 5
	client.RetryWaitMax = time.Second * 35

	req, err := retryablehttp.NewRequestWithContext(
		ctx,
		http.MethodPost,
		proLogic.GetAccountsHost()+"/api/v1/license/validate",
		bytes.NewReader(requestBody),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach netmaker api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read validation response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not validate license with validation backend (status={%d}, body={%s})",
			resp.StatusCode, string(body))
	}

	return body, nil
}

var cachedResponse atomic.Value // ValidatedLicense

func cacheResponse(ctx context.Context, response ValidatedLicense) error {
	cachedResponse.Store(response)

	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return cacheResponseInDB(ctx, raw)
}

var errNoCachedResponse = errors.New("no cached license validation response available")

var errCachedResponseExpired = errors.New("cached license validation response has expired")

func getCachedResponse(ctx context.Context) (ValidatedLicense, error) {
	response, ok := cachedResponse.Load().(ValidatedLicense)
	if !ok {
		raw, err := getCachedResponseFromDB(ctx)
		if err != nil {
			return ValidatedLicense{}, errNoCachedResponse
		}

		if err := json.Unmarshal(raw, &response); err != nil {
			return ValidatedLicense{}, errNoCachedResponse
		}

		cachedResponse.Store(response)
	}

	if !response.Expiry.IsZero() && time.Now().After(response.Expiry) {
		return response, errCachedResponseExpired
	}

	return response, nil
}

func cacheResponseInDB(ctx context.Context, response []byte) error {
	cached := &schema.Internal{
		Key:   schema.InternalKey_LicenseValidationCachedResponse,
		Value: base64encode(response),
	}
	return cached.Set(ctx)
}

func getCachedResponseFromDB(ctx context.Context) ([]byte, error) {
	cached := &schema.Internal{
		Key: schema.InternalKey_LicenseValidationCachedResponse,
	}
	err := cached.Get(ctx)
	if err != nil {
		return nil, err
	}

	return base64decode(cached.Value), nil
}
