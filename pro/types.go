//go:build ee
// +build ee

package pro

import (
	"errors"
)

const (
	license_cache_key          = "license_response_cache"
	license_validation_err_msg = "invalid license"
	server_id_key              = "nm-server-id"
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
	LicenseValue     string `json:"license_value"     binding:"required"` // license that validation is being requested for
	EncryptedLicense string `json:"encrypted_license" binding:"required"` // to be decrypted by Netmaker using Netmaker server's private key
}

// LicenseSecret - the encrypted struct for sending user-id
type LicenseSecret struct {
	AssociatedID string `json:"associated_id" binding:"required"` // UUID for user foreign key to User table
	Usage        Usage  `json:"limits"        binding:"required"`
}

// Usage - struct for license usage
type Usage struct {
	Servers          int `json:"servers"`
	Users            int `json:"users"`
	Hosts            int `json:"hosts"`
	Clients          int `json:"clients"`
	Networks         int `json:"networks"`
	Ingresses        int `json:"ingresses"`
	Egresses         int `json:"egresses"`
	Relays           int `json:"relays"`
	InternetGateways int `json:"internet_gateways"`
	FailOvers        int `json:"fail_overs"`
}

// Usage.SetDefaults - sets the default values for usage
func (l *Usage) SetDefaults() {
	l.Clients = 0
	l.Servers = 1
	l.Hosts = 0
	l.Users = 1
	l.Networks = 0
	l.Ingresses = 0
	l.Egresses = 0
	l.Relays = 0
	l.InternetGateways = 0
}

// ValidateLicenseRequest - used for request to validate license endpoint
type ValidateLicenseRequest struct {
	LicenseKey     string `json:"license_key"       binding:"required"`
	NmServerPubKey string `json:"nm_server_pub_key" binding:"required"` // Netmaker server public key used to send data back to Netmaker for the Netmaker server to decrypt (eg output from validating license)
	EncryptedPart  string `json:"secret"            binding:"required"`
}

type licenseResponseCache struct {
	Body []byte `json:"body" binding:"required"`
}
