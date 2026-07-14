package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func dnsTestCtx() context.Context {
	return scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, defaultTenantID)
}

var dnsHost schema.Host

func TestGetAllDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	createHost()
	t.Run("NoEntries", func(t *testing.T) {
		entries, err := logic.GetAllDNS(dnsTestCtx())
		assert.Nil(t, err)
		assert.Equal(t, []models.DNSEntry(nil), entries)
	})
	t.Run("OneEntry", func(t *testing.T) {
		entry := models.DNSEntry{
			Address: "10.0.0.3", Name: "newhost", Network: "skynet",
		}
		_, err := logic.CreateDNS(dnsTestCtx(), entry)
		assert.Nil(t, err)
		entries, err := logic.GetAllDNS(dnsTestCtx())
		assert.Nil(t, err)
		assert.Equal(t, 1, len(entries))
	})
	t.Run("MultipleEntry", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.7", Name: "anotherhost", Network: "skynet"}
		_, err := logic.CreateDNS(dnsTestCtx(), entry)
		assert.Nil(t, err)
		entries, err := logic.GetAllDNS(dnsTestCtx())
		assert.Nil(t, err)
		assert.Equal(t, 2, len(entries))
	})
}

func TestGetCustomDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	t.Run("NoNetworks", func(t *testing.T) {
		dns, err := logic.GetCustomDNS(dnsTestCtx(), "skynet")
		assert.NoError(t, err)
		assert.Equal(t, []models.DNSEntry(nil), dns)
	})
	t.Run("NoNodes", func(t *testing.T) {
		createNet()
		dns, err := logic.GetCustomDNS(dnsTestCtx(), "skynet")
		assert.NoError(t, err)
		assert.Equal(t, []models.DNSEntry(nil), dns)
	})
	t.Run("NodeExists", func(t *testing.T) {
		createTestNode()
		dns, err := logic.GetCustomDNS(dnsTestCtx(), "skynet")
		assert.NoError(t, err)
		assert.Equal(t, 0, len(dns))
	})
	t.Run("EntryExist", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.3", Name: "custom1", Network: "skynet"}
		_, err := logic.CreateDNS(dnsTestCtx(), entry)
		assert.Nil(t, err)
		dns, err := logic.GetCustomDNS(dnsTestCtx(), "skynet")
		assert.Nil(t, err)
		assert.Equal(t, 1, len(dns))
	})
	t.Run("MultipleEntries", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.4", Name: "host4", Network: "skynet"}
		_, err := logic.CreateDNS(dnsTestCtx(), entry)
		assert.Nil(t, err)
		dns, err := logic.GetCustomDNS(dnsTestCtx(), "skynet")
		assert.Nil(t, err)
		assert.Equal(t, 2, len(dns))
	})
}

func TestGetDNSEntryNum(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		num, err := logic.GetDNSEntryNum(dnsTestCtx(), "myhost", "skynet")
		assert.Nil(t, err)
		assert.Equal(t, 0, num)
	})
	t.Run("NodeExists", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
		_, err := logic.CreateDNS(dnsTestCtx(), entry)
		assert.Nil(t, err)
		num, err := logic.GetDNSEntryNum(dnsTestCtx(), "newhost", "skynet")
		assert.Nil(t, err)
		assert.Equal(t, 1, num)
	})
}
func TestGetDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	t.Run("NoEntries", func(t *testing.T) {
		dns, err := logic.GetDNS(dnsTestCtx(), "skynet")
		assert.Nil(t, err)
		assert.Nil(t, dns)
	})
	t.Run("CustomDNSExists", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
		_, err := logic.CreateDNS(dnsTestCtx(), entry)
		assert.Nil(t, err)
		dns, err := logic.GetDNS(dnsTestCtx(), "skynet")
		t.Log(dns)
		assert.Nil(t, err)
		assert.NotNil(t, dns)
		assert.Equal(t, "skynet", dns[0].Network)
		assert.Equal(t, 1, len(dns))
	})
	t.Run("NodeExists", func(t *testing.T) {
		deleteAllDNS(t)
		createTestNode()
		dns, err := logic.GetDNS(dnsTestCtx(), "skynet")
		assert.Nil(t, err)
		assert.NotNil(t, dns)
		assert.Equal(t, "skynet", dns[0].Network)
		assert.Equal(t, 1, len(dns))
	})
	t.Run("NodeAndCustomDNS", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
		_, err := logic.CreateDNS(dnsTestCtx(), entry)
		assert.Nil(t, err)
		dns, err := logic.GetDNS(dnsTestCtx(), "skynet")
		t.Log(dns)
		assert.Nil(t, err)
		assert.NotNil(t, dns)
		assert.Equal(t, "skynet", dns[0].Network)
		assert.Equal(t, "skynet", dns[1].Network)
		assert.Equal(t, 2, len(dns))
	})
}

func TestCreateDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
	dns, err := logic.CreateDNS(dnsTestCtx(), entry)
	assert.Nil(t, err)
	assert.Equal(t, "newhost", dns.Name)
}

func TestDeleteDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
	_, _ = logic.CreateDNS(dnsTestCtx(), entry)
	t.Run("EntryExists", func(t *testing.T) {
		err := logic.DeleteDNS(dnsTestCtx(), "newhost", "skynet")
		assert.Nil(t, err)
	})
	t.Run("NodeExists", func(t *testing.T) {
		err := logic.DeleteDNS(dnsTestCtx(), "myhost", "skynet")
		assert.Nil(t, err)
	})

	t.Run("NoEntries", func(t *testing.T) {
		err := logic.DeleteDNS(dnsTestCtx(), "myhost", "skynet")
		assert.Nil(t, err)
	})
}

func TestValidateDNSUpdate(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	entry := models.DNSEntry{Address: "10.0.0.2", Name: "myhost", Network: "skynet"}
	t.Run("BadNetwork", func(t *testing.T) {
		change := models.DNSEntry{Address: "10.0.0.2", Name: "myhost", Network: "badnet"}
		err := logic.ValidateDNSUpdate(dnsTestCtx(), change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	t.Run("EmptyNetwork", func(t *testing.T) {
		// this can't actually happen as change.Network is populated if is blank
		change := models.DNSEntry{Address: "10.0.0.2", Name: "myhost"}
		err := logic.ValidateDNSUpdate(dnsTestCtx(), change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	// t.Run("EmptyAddress", func(t *testing.T) {
	// 	//this can't actually happen as change.Address is populated if is blank
	// 	change := models.DNSEntry{"", "", "myhost", "skynet"}
	// 	err := logic.ValidateDNSUpdate(dnsTestCtx(), change, entry)
	// 	assert.NotNil(t, err)
	// 	assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'required' tag")
	// })
	t.Run("BadAddress", func(t *testing.T) {
		change := models.DNSEntry{Address: "10.0.256.1", Name: "myhost", Network: "skynet"}
		err := logic.ValidateDNSUpdate(dnsTestCtx(), change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'ip' tag")
	})
	t.Run("EmptyName", func(t *testing.T) {
		// this can't actually happen as change.Name is populated if is blank
		change := models.DNSEntry{Address: "10.0.0.2", Network: "skynet"}
		err := logic.ValidateDNSUpdate(dnsTestCtx(), change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'required' tag")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		name := ""
		for i := 1; i < 194; i++ {
			name = name + "a"
		}
		change := models.DNSEntry{Address: "10.0.0.2", Name: name, Network: "skynet"}
		err := logic.ValidateDNSUpdate(dnsTestCtx(), change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("NameUnique", func(t *testing.T) {
		change := models.DNSEntry{Address: "10.0.0.2", Name: "myhost", Network: "wirecat"}
		_, _ = logic.CreateDNS(dnsTestCtx(), entry)
		_, _ = logic.CreateDNS(dnsTestCtx(), change)
		err := logic.ValidateDNSUpdate(dnsTestCtx(), change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'name_unique' tag")
		// cleanup
		err = logic.DeleteDNS(dnsTestCtx(), "myhost", "wirecat")
		assert.Nil(t, err)
	})

}
func TestValidateDNSCreate(t *testing.T) {
	_ = logic.DeleteDNS(dnsTestCtx(), "mynode", "skynet")
	t.Run("NoNetwork", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "myhost", Network: "badnet"}
		err := logic.ValidateDNSCreate(dnsTestCtx(), entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	// t.Run("EmptyAddress", func(t *testing.T) {
	// 	entry := models.DNSEntry{"", "", "myhost", "skynet"}
	// 	err := logic.ValidateDNSCreate(dnsTestCtx(), entry)
	// 	assert.NotNil(t, err)
	// 	assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'required' tag")
	// })
	t.Run("BadAddress", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.256.1", Name: "myhost", Network: "skynet"}
		err := logic.ValidateDNSCreate(dnsTestCtx(), entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'ip' tag")
	})
	t.Run("EmptyName", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Network: "skynet"}
		err := logic.ValidateDNSCreate(dnsTestCtx(), entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid input")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		name := ""
		for i := 1; i < 194; i++ {
			name = name + "a"
		}
		entry := models.DNSEntry{Address: "10.0.0.2", Name: name, Network: "skynet"}
		err := logic.ValidateDNSCreate(dnsTestCtx(), entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("NameUnique", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "myhost", Network: "skynet"}
		_, _ = logic.CreateDNS(dnsTestCtx(), entry)
		err := logic.ValidateDNSCreate(dnsTestCtx(), entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'name_unique' tag")
	})
	t.Run("WhiteSpace", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.10.10.5", Name: "white space", Network: "skynet"}
		err := logic.ValidateDNSCreate(dnsTestCtx(), entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid input")
	})
	t.Run("AllSpaces", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.10.10.5", Name: "     ", Network: "skynet"}
		err := logic.ValidateDNSCreate(dnsTestCtx(), entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "invalid input")
	})

}

func createHost() {
	k, _ := wgtypes.ParseKey("DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=")
	dnsHost = schema.Host{
		ID:        uuid.New(),
		PublicKey: schema.WgKey{Key: k.PublicKey()},
		HostPass:  "password",
		OS:        "linux",
		Name:      "dnshost",
	}
	err := logic.CreateHost(dnsTestCtx(), &dnsHost)
	if err != nil {
		fmt.Println("ERROR CREATING HOST", err.Error())
	}
}

func deleteAllDNS(t *testing.T) {
	dns, err := logic.GetAllDNS(dnsTestCtx())
	assert.Nil(t, err)
	for _, record := range dns {
		err := logic.DeleteDNS(dnsTestCtx(), record.Name, record.Network)
		assert.Nil(t, err)
	}
}
