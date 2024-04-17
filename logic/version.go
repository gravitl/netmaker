package logic

import (
	"strings"
	"unicode"

	"github.com/hashicorp/go-version"
)

const MinVersion = "v0.17.0"

// IsVersionComptatible checks that the version passed is compabtible (>=) with MinVersion
func IsVersionComptatible(ver string) bool {
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
