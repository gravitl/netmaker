package schema

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
)

var slugNonAlphaNumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

// generateSlug produces a URL-friendly slug from name with a random 4-digit
// suffix to reduce collisions (e.g. "acme-corp-4821").
func generateSlug(name string) string {
	base := strings.Trim(slugNonAlphaNumericRegex.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "org"
	}
	return fmt.Sprintf("%s-%04d", base, rand.Intn(9000)+1000)
}

const tenantKeySeparator = "::"

// TenantScopedKey composes the physical storage key for a tenant + logical key pair.
func TenantScopedKey(tenantID, key string) string {
	return tenantID + tenantKeySeparator + key
}

// StripTenantKey removes the tenant prefix added by TenantScopedKey, returning
// the original logical key.
func StripTenantKey(tenantID, storedKey string) string {
	return strings.TrimPrefix(storedKey, tenantID+tenantKeySeparator)
}
