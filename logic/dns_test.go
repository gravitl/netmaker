package logic

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestFallbackNameserverNeedsAllUsers(t *testing.T) {
	base := func() *schema.Nameserver {
		return &schema.Nameserver{
			Name:     GooglePublicNameserverName,
			Default:  true,
			Fallback: true,
			Tags:     datatypes.JSONMap{"*": ""},
		}
	}
	if !fallbackNameserverNeedsAllUsers(base()) {
		t.Fatal("pre-user-selector fallback must gain Users[*]")
	}
	withUsers := base()
	withUsers.Users = datatypes.JSONMap{"*": ""}
	if fallbackNameserverNeedsAllUsers(withUsers) {
		t.Fatal("fallback that already targets all users must be left alone")
	}
	specific := base()
	specific.Users = datatypes.JSONMap{"alice": ""}
	if fallbackNameserverNeedsAllUsers(specific) {
		t.Fatal("explicit user assignment must not be replaced with all users")
	}
	notFallback := base()
	notFallback.Fallback = false
	if fallbackNameserverNeedsAllUsers(notFallback) {
		t.Fatal("non-fallback nameservers are not backfilled")
	}
}

func TestNameserverTargetsAll(t *testing.T) {
	allResources := &schema.Nameserver{Tags: datatypes.JSONMap{"*": ""}}
	allUsers := &schema.Nameserver{Users: datatypes.JSONMap{"*": ""}}

	tests := []struct {
		name  string
		ns    *schema.Nameserver
		owner string
		want  bool
	}{
		{"all resources targets node", allResources, "", true},
		{"all resources skips user device", allResources, "alice", false},
		{"all users targets user device", allUsers, "alice", true},
		{"all users skips node", allUsers, "", false},
		{"no all selector", &schema.Nameserver{Tags: datatypes.JSONMap{"net.dev": ""}}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NameserverTargetsAll(tt.ns, tt.owner); got != tt.want {
				t.Fatalf("NameserverTargetsAll() = %v, want %v", got, tt.want)
			}
		})
	}
}
