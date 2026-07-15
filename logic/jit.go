package logic

import (
	"github.com/gravitl/netmaker/schema"
)

var CheckJITAccess = func(string, string) (bool, *schema.JITGrant, error) {
	return true, nil, nil
}

// UserSubjectToNetworkJIT reports whether the user must satisfy JIT for client-app
// extclient creates on the network (JIT enabled + in unscoped / allowlisted groups).
var UserSubjectToNetworkJIT = func(string, *schema.User) bool {
	return false
}
