package controller

import (
	"errors"
	"regexp"
)

var (
	errInvalidExtClientPubKey  = errors.New("incorrect ext client public key")
	errInvalidExtClientID      = errors.New("client ID must be alphanumderic and/or dashes and less that 15 chars")
	errInvalidExtClientExtraIP = errors.New("client extra ip must be a valid cidr")
	errInvalidExtClientDNS     = errors.New("client dns must be a valid ip address")
	errDuplicateExtClientName  = errors.New("duplicate client name")
)

// allow only dashes and alphaneumeric for ext client and node names
func validName(name string) bool {
	reg, err := regexp.Compile("^[a-zA-Z0-9-]+$")
	if err != nil {
		return false
	}
	if !reg.MatchString(name) {
		return false
	}
	if len(name) < 5 || len(name) > 32 {
		return false
	}
	return true
}
