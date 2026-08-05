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
	"github.com/gravitl/netmaker/migrate"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/orchestrator"
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

// AddLicenseHooks - adds the validation hook
func AddLicenseHooks() {
	logic.HookManagerCh <- models.HookDetails{
		ID: "license-validation-hook",
		Hook: logic.WrapHook(func() error {
			return ValidateLicense(db.WithContext(context.Background()), false)
		}),
		Interval: time.Hour,
	}
}

func SyncOrgAndTenants(ctx context.Context) error {
	licenseResponse, isCachedResp, err := fetchValidatedLicense(ctx)
	if err != nil {
		return err
	}

	if isCachedResp {
		return nil
	}

	return syncOrgAndTenantsFromResponse(ctx, licenseResponse, false)
}

func syncOrgAndTenantsFromResponse(ctx context.Context, licenseResponse ValidatedLicense, hadCache bool) error {
	if hadCache {
		return upsertOrgAndTenants(ctx, licenseResponse)
	}
	return reconcileOrgAndTenants(ctx, licenseResponse)
}

func upsertOrgAndTenants(ctx context.Context, licenseResponse ValidatedLicense) error {
	orgID, err := upsertOrganization(ctx, licenseResponse.Organization)
	if err != nil {
		return fmt.Errorf("failed to sync organization: %w", err)
	}

	for _, licenseTenant := range licenseResponse.Tenants {
		if licenseTenant.Status == TenantStatusDeleted {
			if err := teardownDeletedTenant(ctx, licenseTenant.ID); err != nil {
				return fmt.Errorf("failed to tear down deleted tenant %q: %w", licenseTenant.ID, err)
			}
			continue
		}
		if err := upsertTenant(ctx, licenseTenant, orgID); err != nil {
			return fmt.Errorf("failed to sync tenant %q: %w", licenseTenant.ID, err)
		}
	}

	return nil
}

func upsertOrganization(ctx context.Context, licenseOrg LicenseOrg) (string, error) {
	if licenseOrg.ID == "" {
		// single-tenant PRO: the license carries no organization.
		org, err := migrate.EnsureLocalOrganization(ctx)
		if err != nil {
			return "", err
		}
		return org.ID, nil
	}

	org := &schema.Organization{ID: licenseOrg.ID}
	err := org.Get(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		org = &schema.Organization{
			ID:       licenseOrg.ID,
			Name:     licenseOrg.Name,
			Metadata: licenseOrg.Metadata,
		}
		if err := org.Create(ctx); err != nil {
			return "", err
		}
		return org.ID, nil
	}

	return org.ID, nil
}

func upsertTenant(ctx context.Context, licenseTenant LicenseTenant, orgID string) error {
	tenant := &schema.Tenant{ID: licenseTenant.ID}
	err := tenant.Get(ctx)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		tenant = &schema.Tenant{
			ID:             licenseTenant.ID,
			Name:           licenseTenant.Name,
			Metadata:       licenseTenant.Metadata,
			OrganizationID: orgID,
		}
		return orchestrator.GetRepository().TenantOrchestrator().CreateTenant(ctx, tenant)
	}

	return nil
}

func teardownDeletedTenant(ctx context.Context, tenantID string) error {
	tenant := &schema.Tenant{ID: tenantID}
	if err := tenant.Get(ctx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	slog.Warn("tenant marked deleted by license validation -- tearing down local tenant data", "tenant_id", tenantID)
	return orchestrator.GetRepository().TenantOrchestrator().TeardownTenant(ctx, tenantID)
}

func reconcileOrgAndTenants(ctx context.Context, licenseResponse ValidatedLicense) error {
	orgID, err := reconcileOrganization(ctx, licenseResponse.Organization)
	if err != nil {
		return fmt.Errorf("failed to reconcile organization: %w", err)
	}

	if err := reconcileTenants(ctx, licenseResponse.Tenants, orgID); err != nil {
		return fmt.Errorf("failed to reconcile tenants: %w", err)
	}

	return nil
}

func reconcileOrganization(ctx context.Context, licenseOrg LicenseOrg) (string, error) {
	if licenseOrg.ID == "" {
		org, err := migrate.EnsureLocalOrganization(ctx)
		if err != nil {
			return "", err
		}
		return org.ID, nil
	}

	orgs, err := (&schema.Organization{}).ListAll(ctx)
	if err != nil {
		return "", err
	}

	switch len(orgs) {
	case 0:
		org := &schema.Organization{ID: licenseOrg.ID, Name: licenseOrg.Name, Metadata: licenseOrg.Metadata}
		if err := org.Create(ctx); err != nil {
			return "", err
		}
		return org.ID, nil
	case 1:
		existing := orgs[0]
		if existing.ID != licenseOrg.ID {
			if err := migrate.RekeyOrganization(ctx, existing.ID, licenseOrg.ID); err != nil {
				return "", err
			}
		}
		org := &schema.Organization{ID: licenseOrg.ID, Name: licenseOrg.Name, Metadata: licenseOrg.Metadata}
		if err := org.Update(ctx); err != nil {
			return "", err
		}
		return licenseOrg.ID, nil
	default:
		return "", fmt.Errorf("cannot reconcile license organization: %d local organizations already exist", len(orgs))
	}
}

func reconcileTenants(ctx context.Context, licenseTenants []LicenseTenant, orgID string) error {
	existing, err := (&schema.Tenant{}).List(ctx)
	if err != nil {
		return err
	}

	activeTenants := make([]LicenseTenant, 0, len(licenseTenants))
	for _, licenseTenant := range licenseTenants {
		if licenseTenant.Status == TenantStatusDeleted {
			if err := teardownDeletedTenant(ctx, licenseTenant.ID); err != nil {
				return fmt.Errorf("failed to tear down deleted tenant %q: %w", licenseTenant.ID, err)
			}
			continue
		}
		activeTenants = append(activeTenants, licenseTenant)
	}

	switch len(existing) {
	case 0:
		// nothing pre-existing locally -- nothing to rekey.
	case 1:
		if len(activeTenants) != 1 {
			return fmt.Errorf(
				"local tenant %q exists but license returned %d active tenants (expected 1); manual resolution required",
				existing[0].ID, len(activeTenants),
			)
		}
		if existing[0].ID != activeTenants[0].ID {
			if err := migrate.RekeyTenant(ctx, existing[0].ID, activeTenants[0].ID); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("cannot reconcile license tenants: %d local tenants already exist", len(existing))
	}

	for _, licenseTenant := range activeTenants {
		if err := upsertTenant(ctx, licenseTenant, orgID); err != nil {
			return err
		}
	}

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

// ValidateLicense - the initial and periodic license check for netmaker server
// checks if a license is valid + limits are not exceeded
// if license is free_tier and limits exceeds, then function should error
// if license is not valid, function should error
func ValidateLicense(ctx context.Context, clearCache bool) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %s", errValidation, err.Error())
		}
	}()

	hadCache := hasCachedResponse(ctx)

	if clearCache {
		err = clearCachedResponse(ctx)
		if err != nil {
			return err
		}
	}

	licenseResponse, isCachedResp, err := fetchValidatedLicense(ctx)
	if err != nil {
		return err
	}
	if isCachedResp {
		return
	}

	if err = syncOrgAndTenantsFromResponse(ctx, licenseResponse, hadCache); err != nil {
		err = fmt.Errorf("failed to sync organization/tenants from license: %w", err)
		return err
	}

	proLogic.SetDeploymentMode(licenseResponse.DeploymentMode)

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
		if err := cacheResponse(ctx, validationResponse); err != nil {
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

	return licenseResponse, isCachedResp, nil
}

type licenseErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var licenseInvalidErr atomic.Pointer[error]

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
	client.RetryMax = 3
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
		if resp.StatusCode == http.StatusUnauthorized {
			var errResp licenseErrorResponse
			jsonErr := json.Unmarshal(body, &errResp)
			if jsonErr == nil && errResp.Code == http.StatusUnauthorized {
				invalidErr := errors.New(errResp.Message)
				licenseInvalidErr.Store(&invalidErr)
			}
		}
		return nil, fmt.Errorf("could not validate license with validation backend (status={%d})", resp.StatusCode)
	}

	licenseInvalidErr.Store(nil)

	return body, nil
}
