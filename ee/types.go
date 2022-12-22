package ee

import "fmt"

const (
	api_endpoint               = "https://api.controller.netmaker.io/api/v1/license/validate"
	license_cache_key          = "license_response_cache"
	license_validation_err_msg = "invalid license"
	server_id_key              = "nm-server-id"
)

var errValidation = fmt.Errorf(license_validation_err_msg)

// Limits - limits to be referenced throughout server
var Limits = GlobalLimits{
	Servers:  0,
	Users:    0,
	Nodes:    0,
	Clients:  0,
	Networks: 0,
	FreeTier: false,
}

// GlobalLimits - struct for holding global limits on this netmaker server in memory
type GlobalLimits struct {
	Servers  int
	Users    int
	Nodes    int
	Clients  int
	FreeTier bool
	Networks int
}

// LicenseKey - the license key struct representation with associated data
type LicenseKey struct {
	LicenseValue   string `json:"license_value"` // actual (public) key and the unique value for the key
	Expiration     int64  `json:"expiration"`
	LimitServers   int    `json:"limit_servers"`
	LimitUsers     int    `json:"limit_users"`
	LimitNodes     int    `json:"limit_nodes"`
	LimitClients   int    `json:"limit_clients"`
	Metadata       string `json:"metadata"`
	SubscriptionID string `json:"subscription_id"` // for a paid subscription (non-free-tier license)
	FreeTier       string `json:"free_tier"`       // yes if free tier
	IsActive       string `json:"is_active"`       // yes if active
}

// ValidatedLicense - the validated license struct
type ValidatedLicense struct {
	LicenseValue     string `json:"license_value" binding:"required"`     // license that validation is being requested for
	EncryptedLicense string `json:"encrypted_license" binding:"required"` // to be decrypted by Netmaker using Netmaker server's private key
}

// LicenseSecret - the encrypted struct for sending user-id
type LicenseSecret struct {
	UserID string        `json:"user_id" binding:"required"` // UUID for user foreign key to User table
	Limits LicenseLimits `json:"limits" binding:"required"`
}

// LicenseLimits - struct license limits
type LicenseLimits struct {
	Servers int `json:"servers" binding:"required"`
	Users   int `json:"users" binding:"required"`
	Nodes   int `json:"nodes" binding:"required"`
	Clients int `json:"clients" binding:"required"`
}

// LicenseLimits.SetDefaults - sets the default values for limits
func (l *LicenseLimits) SetDefaults() {
	l.Clients = 0
	l.Servers = 1
	l.Nodes = 0
	l.Users = 1
}

// ValidateLicenseRequest - used for request to validate license endpoint
type ValidateLicenseRequest struct {
	NmServerPubKey string `json:"nm_server_pub_key" binding:"required"` // Netmaker server public key used to send data back to Netmaker for the Netmaker server to decrypt (eg output from validating license)
	EncryptedPart  string `json:"secret" binding:"required"`
}

type licenseResponseCache struct {
	Body []byte `json:"body" binding:"required"`
}
