package logic

import (
	"context"

	"github.com/gravitl/netmaker/schema"
)

var CheckJITAccess = func(context.Context, string, string) (bool, *schema.JITGrant, error) {
	return true, nil, nil
}

// UserSubjectToNetworkJIT reports whether the user must satisfy JIT for client-app
// extclient creates on the network (JIT enabled + in unscoped / allowlisted groups).
var UserSubjectToNetworkJIT = func(context.Context, string, *schema.User) bool {
	return false
}
