//go:build ee
// +build ee

package pro

import (
	"encoding/base64"
)

// base64encode - base64 encode helper function
func base64encode(input []byte) string {
	return base64.StdEncoding.EncodeToString(input)
}

// base64decode - base64 decode helper function
func base64decode(input string) []byte {

	bytes, err := base64.StdEncoding.DecodeString(input)

	if err != nil {
		return nil
	}

	return bytes
}
