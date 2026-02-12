package logic

import (
	"github.com/gravitl/netmaker/schema"
)

var CheckJITAccess = func(string, string) (bool, *schema.JITGrant, error) {
	return true, nil, nil
}
