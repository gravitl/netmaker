//go:build ee

package logic

import (
	"errors"
	"testing"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestValidateEgressReq_RejectsVirtualNATForEgressApp(t *testing.T) {
	e := &schema.Egress{
		Network:  "test-net",
		PresetID: "github",
		Nat:      true,
		Mode:     schema.VirtualNAT,
		Domains:  datatypes.JSONSlice[string]{"github.com"},
		Nodes:    datatypes.JSONMap{"node-1": float64(256)},
	}
	err := ValidateEgressReq(e)
	if !errors.Is(err, logic.ErrVirtualNATNotForEgressApps) {
		t.Fatalf("expected ErrVirtualNATNotForEgressApps, got %v", err)
	}
}

func TestValidateEgressReq_ForcesDirectNATForEgressApp(t *testing.T) {
	e := &schema.Egress{
		Network:  "test-net",
		PresetID: "github",
		Nat:      true,
		Mode:     schema.DirectNAT,
		Domains:  datatypes.JSONSlice[string]{"github.com"},
		Nodes:    datatypes.JSONMap{"node-1": float64(256)},
	}
	_ = ValidateEgressReq(e)
	if e.Mode != schema.DirectNAT {
		t.Fatalf("expected direct NAT mode, got %q", e.Mode)
	}
	if e.VirtualRange != "" {
		t.Fatalf("expected empty virtual range, got %q", e.VirtualRange)
	}
}
