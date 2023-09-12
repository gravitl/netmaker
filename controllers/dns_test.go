package controller

import (
	"net"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/txn2/txeh"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

var dnsHost models.Host

func TestGetAllDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	createHost()
	t.Run("NoEntries", func(t *testing.T) {
		entries, err := logic.GetAllDNS()
		assert.Nil(t, err)
		assert.Equal(t, []models.DNSEntry(nil), entries)
	})
	t.Run("OneEntry", func(t *testing.T) {
		entry := models.DNSEntry{
			Address: "10.0.0.3", Name: "newhost", Network: "skynet",
		}
		_, err := logic.CreateDNS(entry)
		assert.Nil(t, err)
		entries, err := logic.GetAllDNS()
		assert.Nil(t, err)
		assert.Equal(t, 1, len(entries))
	})
	t.Run("MultipleEntry", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.7", Name: "anotherhost", Network: "skynet"}
		_, err := logic.CreateDNS(entry)
		assert.Nil(t, err)
		entries, err := logic.GetAllDNS()
		assert.Nil(t, err)
		assert.Equal(t, 2, len(entries))
	})
}

func TestGetNodeDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	createHost()
	err := functions.SetDNSDir()
	assert.Nil(t, err)
	t.Run("NoNodes", func(t *testing.T) {
		dns, _ := logic.GetNodeDNS("skynet")
		assert.Equal(t, []models.DNSEntry(nil), dns)
	})
	t.Run("NodeExists", func(t *testing.T) {
		createHost()
		_, ipnet, _ := net.ParseCIDR("10.0.0.1/32")
		tmpCNode := models.CommonNode{
			ID:      uuid.New(),
			Network: "skynet",
			Address: *ipnet,
			DNSOn:   true,
		}
		createnode := models.Node{
			CommonNode: tmpCNode,
		}
		err := logic.AssociateNodeToHost(&createnode, &dnsHost)
		assert.Nil(t, err)
		dns, err := logic.GetNodeDNS("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "10.0.0.1", dns[0].Address)
	})
	t.Run("MultipleNodes", func(t *testing.T) {
		_, ipnet, _ := net.ParseCIDR("10.100.100.3/32")
		tmpCNode := models.CommonNode{
			ID:      uuid.New(),
			Network: "skynet",
			Address: *ipnet,
			DNSOn:   true,
		}
		createnode := models.Node{
			CommonNode: tmpCNode,
		}
		err := logic.AssociateNodeToHost(&createnode, &dnsHost)
		assert.Nil(t, err)
		dns, err := logic.GetNodeDNS("skynet")
		assert.Nil(t, err)
		assert.Equal(t, 2, len(dns))
	})
}
func TestGetCustomDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	t.Run("NoNetworks", func(t *testing.T) {
		dns, err := logic.GetCustomDNS("skynet")
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, []models.DNSEntry(nil), dns)
	})
	t.Run("NoNodes", func(t *testing.T) {
		createNet()
		dns, err := logic.GetCustomDNS("skynet")
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, []models.DNSEntry(nil), dns)
	})
	t.Run("NodeExists", func(t *testing.T) {
		createTestNode()
		dns, err := logic.GetCustomDNS("skynet")
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, 0, len(dns))
	})
	t.Run("EntryExist", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.3", Name: "custom1", Network: "skynet"}
		_, err := logic.CreateDNS(entry)
		assert.Nil(t, err)
		dns, err := logic.GetCustomDNS("skynet")
		assert.Nil(t, err)
		assert.Equal(t, 1, len(dns))
	})
	t.Run("MultipleEntries", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.4", Name: "host4", Network: "skynet"}
		_, err := logic.CreateDNS(entry)
		assert.Nil(t, err)
		dns, err := logic.GetCustomDNS("skynet")
		assert.Nil(t, err)
		assert.Equal(t, 2, len(dns))
	})
}

func TestGetDNSEntryNum(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		num, err := logic.GetDNSEntryNum("myhost", "skynet")
		assert.Nil(t, err)
		assert.Equal(t, 0, num)
	})
	t.Run("NodeExists", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
		_, err := logic.CreateDNS(entry)
		assert.Nil(t, err)
		num, err := logic.GetDNSEntryNum("newhost", "skynet")
		assert.Nil(t, err)
		assert.Equal(t, 1, num)
	})
}
func TestGetDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	t.Run("NoEntries", func(t *testing.T) {
		dns, err := logic.GetDNS("skynet")
		assert.Nil(t, err)
		assert.Nil(t, dns)
	})
	t.Run("CustomDNSExists", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
		_, err := logic.CreateDNS(entry)
		assert.Nil(t, err)
		dns, err := logic.GetDNS("skynet")
		t.Log(dns)
		assert.Nil(t, err)
		assert.NotNil(t, dns)
		assert.Equal(t, "skynet", dns[0].Network)
		assert.Equal(t, 1, len(dns))
	})
	t.Run("NodeExists", func(t *testing.T) {
		deleteAllDNS(t)
		createTestNode()
		dns, err := logic.GetDNS("skynet")
		assert.Nil(t, err)
		assert.NotNil(t, dns)
		assert.Equal(t, "skynet", dns[0].Network)
		assert.Equal(t, 1, len(dns))
	})
	t.Run("NodeAndCustomDNS", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
		_, err := logic.CreateDNS(entry)
		assert.Nil(t, err)
		dns, err := logic.GetDNS("skynet")
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
	dns, err := logic.CreateDNS(entry)
	assert.Nil(t, err)
	assert.Equal(t, "newhost", dns.Name)
}

func TestSetDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	etc, err := txeh.NewHosts(&txeh.HostsConfig{})
	assert.Nil(t, err)
	err = functions.SetDNSDir()
	assert.Nil(t, err)
	t.Run("NoNetworks", func(t *testing.T) {
		err := logic.SetDNS()
		assert.Nil(t, err)
		info, err := txeh.NewHosts(&txeh.HostsConfig{
			ReadFilePath: "./config/dnsconfig/netmaker.hosts",
		})
		assert.Nil(t, err)
		assert.Equal(t, etc.RenderHostsFile(), info.RenderHostsFile())
	})
	t.Run("NoEntries", func(t *testing.T) {
		createNet()
		err := logic.SetDNS()
		assert.Nil(t, err)
		info, err := txeh.NewHosts(&txeh.HostsConfig{
			ReadFilePath: "./config/dnsconfig/netmaker.hosts",
		})
		assert.Nil(t, err)
		assert.Equal(t, etc.RenderHostsFile(), info.RenderHostsFile())
	})
	t.Run("NodeExists", func(t *testing.T) {
		createTestNode()
		err := logic.SetDNS()
		assert.Nil(t, err)
		info, err := os.Stat("./config/dnsconfig/netmaker.hosts")
		assert.Nil(t, err)
		assert.False(t, info.IsDir())
		content, err := os.ReadFile("./config/dnsconfig/netmaker.hosts")
		assert.Nil(t, err)
		assert.Contains(t, string(content), "linuxhost.skynet")
	})
	t.Run("EntryExists", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.3", Name: "newhost", Network: "skynet"}
		_, err := logic.CreateDNS(entry)
		assert.Nil(t, err)
		err = logic.SetDNS()
		assert.Nil(t, err)
		info, err := os.Stat("./config/dnsconfig/netmaker.hosts")
		assert.Nil(t, err)
		assert.False(t, info.IsDir())
		content, err := os.ReadFile("./config/dnsconfig/netmaker.hosts")
		assert.Nil(t, err)
		assert.Contains(t, string(content), "newhost.skynet")
	})

}

func TestGetDNSEntry(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	createTestNode()
	entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
	_, _ = logic.CreateDNS(entry)
	t.Run("wrong net", func(t *testing.T) {
		entry, err := GetDNSEntry("newhost", "w286 Toronto Street South, Uxbridge, ONirecat")
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, models.DNSEntry{}, entry)
	})
	t.Run("wrong host", func(t *testing.T) {
		entry, err := GetDNSEntry("badhost", "skynet")
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, models.DNSEntry{}, entry)
	})
	t.Run("good host", func(t *testing.T) {
		entry, err := GetDNSEntry("newhost", "skynet")
		assert.Nil(t, err)
		assert.Equal(t, "newhost", entry.Name)
	})
	t.Run("node", func(t *testing.T) {
		entry, err := GetDNSEntry("testnode", "skynet")
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, models.DNSEntry{}, entry)
	})
}

func TestDeleteDNS(t *testing.T) {
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	entry := models.DNSEntry{Address: "10.0.0.2", Name: "newhost", Network: "skynet"}
	_, _ = logic.CreateDNS(entry)
	t.Run("EntryExists", func(t *testing.T) {
		err := logic.DeleteDNS("newhost", "skynet")
		assert.Nil(t, err)
	})
	t.Run("NodeExists", func(t *testing.T) {
		err := logic.DeleteDNS("myhost", "skynet")
		assert.Nil(t, err)
	})

	t.Run("NoEntries", func(t *testing.T) {
		err := logic.DeleteDNS("myhost", "skynet")
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
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	t.Run("EmptyNetwork", func(t *testing.T) {
		// this can't actually happen as change.Network is populated if is blank
		change := models.DNSEntry{Address: "10.0.0.2", Name: "myhost"}
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	// t.Run("EmptyAddress", func(t *testing.T) {
	// 	//this can't actually happen as change.Address is populated if is blank
	// 	change := models.DNSEntry{"", "", "myhost", "skynet"}
	// 	err := logic.ValidateDNSUpdate(change, entry)
	// 	assert.NotNil(t, err)
	// 	assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'required' tag")
	// })
	t.Run("BadAddress", func(t *testing.T) {
		change := models.DNSEntry{Address: "10.0.256.1", Name: "myhost", Network: "skynet"}
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'ip' tag")
	})
	t.Run("EmptyName", func(t *testing.T) {
		// this can't actually happen as change.Name is populated if is blank
		change := models.DNSEntry{Address: "10.0.0.2", Network: "skynet"}
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'required' tag")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		name := ""
		for i := 1; i < 194; i++ {
			name = name + "a"
		}
		change := models.DNSEntry{Address: "10.0.0.2", Name: name, Network: "skynet"}
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("NameUnique", func(t *testing.T) {
		change := models.DNSEntry{Address: "10.0.0.2", Name: "myhost", Network: "wirecat"}
		_, _ = logic.CreateDNS(entry)
		_, _ = logic.CreateDNS(change)
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'name_unique' tag")
		// cleanup
		err = logic.DeleteDNS("myhost", "wirecat")
		assert.Nil(t, err)
	})

}
func TestValidateDNSCreate(t *testing.T) {
	_ = logic.DeleteDNS("mynode", "skynet")
	t.Run("NoNetwork", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "myhost", Network: "badnet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	// t.Run("EmptyAddress", func(t *testing.T) {
	// 	entry := models.DNSEntry{"", "", "myhost", "skynet"}
	// 	err := logic.ValidateDNSCreate(entry)
	// 	assert.NotNil(t, err)
	// 	assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'required' tag")
	// })
	t.Run("BadAddress", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.256.1", Name: "myhost", Network: "skynet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'ip' tag")
	})
	t.Run("EmptyName", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Network: "skynet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'required' tag")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		name := ""
		for i := 1; i < 194; i++ {
			name = name + "a"
		}
		entry := models.DNSEntry{Address: "10.0.0.2", Name: name, Network: "skynet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("NameUnique", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.0.0.2", Name: "myhost", Network: "skynet"}
		_, _ = logic.CreateDNS(entry)
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'name_unique' tag")
	})
	t.Run("WhiteSpace", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.10.10.5", Name: "white space", Network: "skynet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'whitespace' tag")
	})
	t.Run("AllSpaces", func(t *testing.T) {
		entry := models.DNSEntry{Address: "10.10.10.5", Name: "     ", Network: "skynet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'whitespace' tag")
	})

}

func createHost() {
	k, _ := wgtypes.ParseKey("DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=")
	dnsHost = models.Host{
		ID:        uuid.New(),
		PublicKey: k.PublicKey(),
		HostPass:  "password",
		OS:        "linux",
		Name:      "dnshost",
	}
	_ = logic.CreateHost(&dnsHost)
}

func deleteAllDNS(t *testing.T) {
	dns, err := logic.GetAllDNS()
	assert.Nil(t, err)
	for _, record := range dns {
		err := logic.DeleteDNS(record.Name, record.Network)
		assert.Nil(t, err)
	}
}
