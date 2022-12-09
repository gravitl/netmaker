package controller

import (
	"errors"
	"regexp"
)

var (
	errInvalidNodeName    = errors.New("Node name must be alphanumderic and/or dashes")
	errInvalidExtClientID = errors.New("Ext client ID must be alphanumderic and/or dashes")
)

// allow only dashes and alphaneumeric for ext client and node names
func validName(name string) bool {
	return regexp.MustCompile("^[a-zA-Z0-9-]+$").MatchString(name)
}
