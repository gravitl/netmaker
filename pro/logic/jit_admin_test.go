package logic

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestGroupGrantsNetworkAdminOn(t *testing.T) {
	netID := schema.NetworkID("testnet")
	adminRole := GetDefaultNetworkAdminRoleID(netID)

	t.Run("network_scoped_admin_role", func(t *testing.T) {
		g := &schema.UserGroup{
			NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
				netID: {adminRole: {}},
			}),
		}
		if !groupGrantsNetworkAdminOn(g, netID) {
			t.Fatal("expected network admin grant")
		}
	})

	t.Run("generic_network_admin_constant_not_sufficient", func(t *testing.T) {
		g := &schema.UserGroup{
			NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
				netID: {schema.NetworkAdmin: {}},
			}),
		}
		if groupGrantsNetworkAdminOn(g, netID) {
			t.Fatal("schema.NetworkAdmin is not the stored role id")
		}
	})
}

func TestGroupGrantsGlobalNetworkAdmin(t *testing.T) {
	t.Run("all_networks_admin_group_role", func(t *testing.T) {
		g := &schema.UserGroup{
			NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
				schema.AllNetworks: {globalNetworksAdminRoleID: {}},
			}),
		}
		if !groupGrantsGlobalNetworkAdmin(g) {
			t.Fatal("expected global network admin grant")
		}
	})

	t.Run("generic_network_admin_constant_not_sufficient", func(t *testing.T) {
		g := &schema.UserGroup{
			NetworkRoles: datatypes.NewJSONType(schema.NetworkRoles{
				schema.AllNetworks: {schema.NetworkAdmin: {}},
			}),
		}
		if groupGrantsGlobalNetworkAdmin(g) {
			t.Fatal("schema.NetworkAdmin is not the stored role id under all networks")
		}
	})
}
