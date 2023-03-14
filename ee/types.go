package ee

import "fmt"

const (
	api_endpoint               = "https://188b-2a00-f28-ff05-4e7f-bcd8-150f-1d1-64e7.in.ngrok.io/api/v1/license/validate"
	license_cache_key          = "license_response_cache"
	license_validation_err_msg = "invalid license"
	server_id_key              = "nm-server-id"
)

var errValidation = fmt.Errorf(license_validation_err_msg)

// Limits - limits to be referenced throughout server
var Limits = GlobalLimits{
	Servers:  0,
	Users:    0,
	Hosts:    0,
	Clients:  0,
	Networks: 0,
}

// GlobalLimits - struct for holding global limits on this netmaker server in memory
type GlobalLimits struct {
	Servers  int
	Users    int
	Hosts    int
	Clients  int
	Networks int
}

// LicenseKey - the license key struct representation with associated data
type LicenseKey struct {
	LicenseValue  string `json:"license_value"` // actual (public) key and the unique value for the key
	Expiration    int64  `json:"expiration"`
	LimitServers  int    `json:"limit_servers"`
	LimitUsers    int    `json:"limit_users"`
	LimitHosts    int    `json:"limit_hosts"`
	LimitNetworks int    ` json:"limit_networks"`
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
	Servers int `json:"servers" binding:"required"`
	Users   int `json:"users" binding:"required"`
	Hosts   int `json:"hosts" binding:"required"`
	Clients int `json:"clients" binding:"required"`
}

// LicenseLimits.SetDefaults - sets the default values for limits
func (l *LicenseLimits) SetDefaults() {
	l.Clients = 0
	l.Servers = 1
	l.Hosts = 0
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
