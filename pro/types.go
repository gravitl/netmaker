//go:build ee
// +build ee

package pro

import (
	"fmt"
	"github.com/gravitl/netmaker/models"
)

// constants for accounts api hosts
const (
	// accountsHostDevelopment is the accounts api host for development environment
	accountsHostDevelopment = "https://api.dev.accounts.netmaker.io"
	// accountsHostStaging is the accounts api host for staging environment
	accountsHostStaging = "https://api.staging.accounts.netmaker.io"
	// accountsHostProduction is the accounts api host for production environment
	accountsHostProduction = "https://api.accounts.netmaker.io"
)

const (
	license_cache_key          = "license_response_cache"
	license_validation_err_msg = "invalid license"
	server_id_key              = "nm-server-id"
)

var errValidation = fmt.Errorf(license_validation_err_msg)

// LicenseKey - the license key struct representation with associated data
type LicenseKey struct {
	LicenseValue   string `json:"license_value"` // actual (public) key and the unique value for the key
	Expiration     int64  `json:"expiration"`
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
	LicenseValue     string `json:"license_value" binding:"required"`     // license that validation is being requested for
	EncryptedLicense string `json:"encrypted_license" binding:"required"` // to be decrypted by Netmaker using Netmaker server's private key
}

// LicenseSecret - the encrypted struct for sending user-id
type LicenseSecret struct {
	AssociatedID string       `json:"associated_id" binding:"required"` // UUID for user foreign key to User table
	Usage        models.Usage `json:"limits" binding:"required"`
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
