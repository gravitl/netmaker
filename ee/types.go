package ee

import "fmt"

const (
	api_endpoint               = "https://api.staging.accounts.netmaker.io/api/v1/license/validate"
	license_cache_key          = "license_response_cache"
	license_validation_err_msg = "invalid license"
	server_id_key              = "nm-server-id"
)

var errValidation = fmt.Errorf(license_validation_err_msg)

// LicenseKey - the license key struct representation with associated data
type LicenseKey struct {
	LicenseValue  string `json:"license_value"` // actual (public) key and the unique value for the key
	Expiration    int64  `json:"expiration"`
	LimitServers  int    `json:"limit_servers"`
	LimitUsers    int    `json:"limit_users"`
	LimitHosts    int    `json:"limit_hosts"`
	LimitNetworks int    `json:"limit_networks"`
	LimitClients  int    `json:"limit_clients"`
	Metadata      string `json:"metadata"`
	IsActive      bool   `json:"is_active"` // yes if active
}

// ValidatedLicense - the validated license struct
type ValidatedLicense struct {
	LicenseValue     string `json:"license_value" binding:"required"`     // license that validation is being requested for
	EncryptedLicense string `json:"encrypted_license" binding:"required"` // to be decrypted by Netmaker using Netmaker server's private key
}

// LicenseSecret - the encrypted struct for sending user-id
type LicenseSecret struct {
	AssociatedID string        `json:"associated_id" binding:"required"` // UUID for user foreign key to User table
	Limits       LicenseLimits `json:"limits" binding:"required"`
}

// LicenseLimits - struct license limits
type LicenseLimits struct {
	Servers  int `json:"servers"`
	Users    int `json:"users"`
	Hosts    int `json:"hosts"`
	Clients  int `json:"clients"`
	Networks int `json:"networks"`
}

// LicenseLimits.SetDefaults - sets the default values for limits
func (l *LicenseLimits) SetDefaults() {
	l.Clients = 0
	l.Servers = 1
	l.Hosts = 0
	l.Users = 1
	l.Networks = 0
}

// ValidateLicenseRequest - used for request to validate license endpoint
type ValidateLicenseRequest struct {
	LicenseKey     string `json:"license_key" binding:"required"`
	NmServerPubKey string `json:"nm_server_pub_key" binding:"required"` // Netmaker server public key used to send data back to Netmaker for the Netmaker server to decrypt (eg output from validating license)
	EncryptedPart  string `json:"secret" binding:"required"`
}

type licenseResponseCache struct {
	Body []byte `json:"body" binding:"required"`
}
