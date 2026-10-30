package logic

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestNetworkHasOtherEnabledFallback(t *testing.T) {
	list := []schema.Nameserver{
		{ID: "a", Fallback: true, Status: true},
		{ID: "b", Fallback: true, Status: false},
		{ID: "c", Fallback: false, Status: true},
	}
	if !networkHasOtherEnabledFallback(list, "b") {
		t.Fatal("enabled fallback a must count when excluding b")
	}
	if networkHasOtherEnabledFallback(list, "a") {
		t.Fatal("no other enabled fallback when excluding a")
	}
	if !networkHasOtherEnabledFallback(list, "") {
		t.Fatal("enabled fallback a must count when excluding none")
	}
}

func TestWouldLoseLastEnabledFallback(t *testing.T) {
	enabled := &schema.Nameserver{ID: "a", Fallback: true, Status: true}
	if !wouldLoseLastEnabledFallback(enabled, &schema.Nameserver{Fallback: true, Status: false}, false) {
		t.Fatal("disabling the only enabled fallback must be blocked")
	}
	if !wouldLoseLastEnabledFallback(enabled, &schema.Nameserver{Fallback: false, Status: true}, false) {
		t.Fatal("unsetting Fallback on the only enabled fallback must be blocked")
	}
	if wouldLoseLastEnabledFallback(enabled, &schema.Nameserver{Fallback: true, Status: false}, true) {
		t.Fatal("disabling is allowed when another enabled fallback remains")
	}
	if wouldLoseLastEnabledFallback(enabled, &schema.Nameserver{Fallback: true, Status: true}, false) {
		t.Fatal("keeping an enabled fallback must be allowed")
	}
	disabled := &schema.Nameserver{ID: "a", Fallback: true, Status: false}
	if wouldLoseLastEnabledFallback(disabled, &schema.Nameserver{Fallback: true, Status: false}, false) {
		t.Fatal("already-disabled fallback is not the last enabled")
	}
	def := &schema.Nameserver{ID: "a", Default: true, Fallback: true, Status: true}
	if !wouldLoseLastEnabledFallback(def, &schema.Nameserver{Fallback: false, Status: false}, false) {
		t.Fatal("disabling default fallback status must be blocked when it is the last")
	}
	// Default keeps Fallback even if the request clears the flag.
	if wouldLoseLastEnabledFallback(def, &schema.Nameserver{Fallback: false, Status: true}, false) {
		t.Fatal("clearing Fallback on a default nameserver is ignored while Status stays on")
	}
}

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
