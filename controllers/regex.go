package controller

import (
	"errors"
	"regexp"
)

var errInvalidExtClientID = errors.New("ext client ID must be alphanumderic and/or dashes")

// allow only dashes and alphaneumeric for ext client and node names
func validName(name string) bool {
	return regexp.MustCompile("^[a-zA-Z0-9-]+$").MatchString(name)
}
