package logic

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/hashicorp/go-version"
)

const MinVersion = "v0.17.0"

// IsVersionCompatible checks that the version passed is compabtible (>=) with MinVersion
func IsVersionCompatible(ver string) bool {
	// during dev, assume developers know what they are doing
	if ver == "dev" {
		return true
	}
	trimmed := strings.TrimFunc(ver, func(r rune) bool {
		return !unicode.IsNumber(r)
	})
	v, err := version.NewVersion(trimmed)
	if err != nil {
		return false
	}
	constraint, err := version.NewConstraint(">= " + MinVersion)
	if err != nil {
		return false
	}
	return constraint.Check(v)

}

// CleanVersion normalizes a version string safely for storage.
// - removes "v" or "V" prefix
// - trims whitespace
// - strips invalid trailing characters
// - preserves semver, prerelease, and build metadata
func CleanVersion(raw string) string {
	if raw == "" {
		return ""
	}

	v := strings.TrimSpace(raw)

	// Remove leading v/V (common in semver)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")

	// Remove trailing commas, quotes, spaces
	v = strings.Trim(v, " ,\"'")

	// Remove any characters not allowed in semantic versioning:
	// Allowed: 0-9 a-z A-Z . - +
	re := regexp.MustCompile(`[^0-9A-Za-z\.\-\+]+`)
	v = re.ReplaceAllString(v, "")

	// Collapse multiple dots (e.g., "1..2" → "1.2")
	v = strings.ReplaceAll(v, "..", ".")
	for strings.Contains(v, "..") {
		v = strings.ReplaceAll(v, "..", ".")
	}

	return v
}
