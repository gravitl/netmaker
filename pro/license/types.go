package license

import (
	"errors"

	"github.com/gravitl/netmaker/models"
)

const (
	license_validation_err_msg = "invalid license"
)

var errValidation = errors.New(license_validation_err_msg)

// LicenseKey - the license key struct representation with associated data
type LicenseKey struct {
	LicenseValue   string `json:"license_value"` // actual (public) key and the unique value for the key
	Expiration     int64  `json:"expiration" swaggertype:"primitive,integer" format:"int64"`
	UsageServers   int    `json:"limit_servers"`
	UsageUsers     int    `json:"limit_users"`
	UsageClients   int    `json:"limit_clients"`
	UsageHosts     int    `json:"limit_hosts"`
	UsageNetworks  int    `json:"limit_networks"`
	UsageIngresses int    `json:"limit_ingresses"`
	UsageEgresses  int    `json:"limit_egresses"`
	Metadata       string `json:"metadata"`
	IsActive       bool   `json:"is_active"` // yes if active
}

// ValidatedLicense - the validated license struct
type ValidatedLicense struct {
	LicenseValue     string              `json:"license_value"     binding:"required"` // license that validation is being requested for
	EncryptedLicense string              `json:"encrypted_license" binding:"required"` // to be decrypted by Netmaker using Netmaker server's private key
	DeploymentMode   string              `json:"deployment_mode"`
	FeatureFlags     models.FeatureFlags `json:"feature_flags" binding:"required"`
	Organization     LicenseOrg          `json:"organization"`
	Tenants          []LicenseTenant     `json:"tenants"`
}

type LicenseOrg struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Metadata string `json:"metadata"`
	Status   string `json:"status"`
}

type LicenseTenant struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Metadata      string              `json:"metadata"`
	Status        TenantStatusMessage `json:"status"`
	EnforceLimits bool                `json:"enforce_limits"`
	Limits        Limits              `json:"limits"`
	FeatureFlags  models.FeatureFlags `json:"feature_flags"`
}

// TenantStatusMessage is an enumeration of the status of the tenant,
// represented as string message codes.
type TenantStatusMessage string

const (
	// TenantStatusOk indicates all is fine with the tenant status
	TenantStatusOk TenantStatusMessage = "ok"
	// TenantStatusPaymentInvalid indicates the user provided an invalid payment method (payment failed too many times)
	TenantStatusPaymentInvalid TenantStatusMessage = "payment_invalid"
	// TenantStatusPaymentMissing indicates the user has not provided a payment method
	TenantStatusPaymentMissing TenantStatusMessage = "payment_missing"
	// TenantStatusLicenseExpired indicates that the tenant's license has expired
	TenantStatusLicenseExpired TenantStatusMessage = "license_expired"
)

type Limits struct {
	Servers   int `json:"servers"`
	Users     int `json:"users"`
	Clients   int `json:"clients"`
	Hosts     int `json:"hosts"`
	Networks  int `json:"networks"`
	Machines  int `json:"machines"`
	Ingresses int `json:"ingresses"`
	Egresses  int `json:"egresses"`
}

// LicenseSecret - the encrypted struct for sending user-id
type LicenseSecret struct {
	AssociatedID string                  `json:"associated_id" binding:"required"` // UUID for user foreign key to User table
	Usage        models.Usage            `json:"limits"        binding:"required"`
	TenantUsage  map[string]models.Usage `json:"tenant_usage"`
}

// ValidateLicenseRequest - used for request to validate license endpoint
type ValidateLicenseRequest struct {
	ServerVersion  string `json:"server_version"`
	LicenseKey     string `json:"license_key"       binding:"required"`
	NmServerPubKey string `json:"nm_server_pub_key" binding:"required"` // Netmaker server public key used to send data back to Netmaker for the Netmaker server to decrypt (eg output from validating license)
	EncryptedPart  string `json:"secret"            binding:"required"`
	NmBaseDomain   string `json:"nm_base_domain"`
}
