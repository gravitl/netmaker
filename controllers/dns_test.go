package controller

import (
	"os"
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestGetAllDNS(t *testing.T) {
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	t.Run("NoEntries", func(t *testing.T) {
		entries, err := logic.GetAllDNS()
		assert.Nil(t, err)
		assert.Equal(t, []models.DNSEntry(nil), entries)
	})
	t.Run("OneEntry", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.3", "", "newhost", "skynet"}
		CreateDNS(entry)
		entries, err := logic.GetAllDNS()
		assert.Nil(t, err)
		assert.Equal(t, 1, len(entries))
	})
	t.Run("MultipleEntry", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.7", "", "anotherhost", "skynet"}
		CreateDNS(entry)
		entries, err := logic.GetAllDNS()
		assert.Nil(t, err)
		assert.Equal(t, 2, len(entries))
	})
}

func TestGetNodeDNS(t *testing.T) {
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		dns, err := logic.GetNodeDNS("skynet")
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, []models.DNSEntry(nil), dns)
	})
	t.Run("NodeExists", func(t *testing.T) {
		createnode := models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Name: "testnode", Endpoint: "10.0.0.1", MacAddress: "01:02:03:04:05:06", Password: "password", Network: "skynet", OS: "linux", DNSOn: "yes"}
		err := logic.CreateNode(&createnode)
		assert.Nil(t, err)
		dns, err := logic.GetNodeDNS("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "10.0.0.1", dns[0].Address)
	})
	t.Run("MultipleNodes", func(t *testing.T) {
		createnode := &models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.100.100.3", MacAddress: "01:02:03:04:05:07", Password: "password", Network: "skynet"}
		err := logic.CreateNode(createnode)
		assert.Nil(t, err)
		dns, err := logic.GetNodeDNS("skynet")
		assert.Nil(t, err)
		assert.Equal(t, 2, len(dns))
	})
}
func TestGetCustomDNS(t *testing.T) {
	database.InitializeDatabase()
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
		entry := models.DNSEntry{"10.0.0.3", "", "newhost", "skynet"}
		CreateDNS(entry)
		dns, err := logic.GetCustomDNS("skynet")
		assert.Nil(t, err)
		assert.Equal(t, 1, len(dns))
	})
	t.Run("MultipleEntries", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.4", "", "host4", "skynet"}
		CreateDNS(entry)
		dns, err := logic.GetCustomDNS("skynet")
		assert.Nil(t, err)
		assert.Equal(t, 2, len(dns))
	})
}

func TestGetDNSEntryNum(t *testing.T) {
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		num, err := logic.GetDNSEntryNum("myhost", "skynet")
		assert.Nil(t, err)
		assert.Equal(t, 0, num)
	})
	t.Run("NodeExists", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "", "newhost", "skynet"}
		_, err := CreateDNS(entry)
		assert.Nil(t, err)
		num, err := logic.GetDNSEntryNum("newhost", "skynet")
		assert.Nil(t, err)
		assert.Equal(t, 1, num)
	})
}
func TestGetDNS(t *testing.T) {
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	t.Run("NoEntries", func(t *testing.T) {
		dns, err := logic.GetDNS("skynet")
		assert.Nil(t, err)
		assert.Nil(t, dns)
	})
	t.Run("CustomDNSExists", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "", "newhost", "skynet"}
		_, err := CreateDNS(entry)
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
		entry := models.DNSEntry{"10.0.0.2", "", "newhost", "skynet"}
		_, err := CreateDNS(entry)
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
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	entry := models.DNSEntry{"10.0.0.2", "", "newhost", "skynet"}
	dns, err := CreateDNS(entry)
	assert.Nil(t, err)
	assert.Equal(t, "newhost", dns.Name)
}

func TestSetDNS(t *testing.T) {
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	t.Run("NoNetworks", func(t *testing.T) {
		err := logic.SetDNS()
		assert.Nil(t, err)
		info, err := os.Stat("./config/dnsconfig/netmaker.hosts")
		assert.Nil(t, err)
		assert.False(t, info.IsDir())
		assert.Equal(t, int64(0), info.Size())
	})
	t.Run("NoEntries", func(t *testing.T) {
		createNet()
		err := logic.SetDNS()
		assert.Nil(t, err)
		info, err := os.Stat("./config/dnsconfig/netmaker.hosts")
		assert.Nil(t, err)
		assert.False(t, info.IsDir())
		assert.Equal(t, int64(0), info.Size())
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
		assert.Contains(t, string(content), "testnode.skynet")
	})
	t.Run("EntryExists", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.3", "", "newhost", "skynet"}
		CreateDNS(entry)
		err := logic.SetDNS()
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
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	createTestNode()
	entry := models.DNSEntry{"10.0.0.2", "", "newhost", "skynet"}
	CreateDNS(entry)
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

//	func TestUpdateDNS(t *testing.T) {
//		var newentry models.DNSEntry
//		database.InitializeDatabase()
//		deleteAllDNS(t)
//		deleteAllNetworks()
//		createNet()
//		entry := models.DNSEntry{"10.0.0.2", "newhost", "skynet"}
//		CreateDNS(entry)
//		t.Run("change address", func(t *testing.T) {
//			newentry.Address = "10.0.0.75"
//			updated, err := UpdateDNS(newentry, entry)
//			assert.Nil(t, err)
//			assert.Equal(t, newentry.Address, updated.Address)
//		})
//		t.Run("change name", func(t *testing.T) {
//			newentry.Name = "newname"
//			updated, err := UpdateDNS(newentry, entry)
//			assert.Nil(t, err)
//			assert.Equal(t, newentry.Name, updated.Name)
//		})
//		t.Run("change network", func(t *testing.T) {
//			newentry.Network = "wirecat"
//			updated, err := UpdateDNS(newentry, entry)
//			assert.Nil(t, err)
//			assert.NotEqual(t, newentry.Network, updated.Network)
//		})
//	}
func TestDeleteDNS(t *testing.T) {
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	entry := models.DNSEntry{"10.0.0.2", "", "newhost", "skynet"}
	CreateDNS(entry)
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
	database.InitializeDatabase()
	deleteAllDNS(t)
	deleteAllNetworks()
	createNet()
	entry := models.DNSEntry{"10.0.0.2", "", "myhost", "skynet"}
	t.Run("BadNetwork", func(t *testing.T) {
		change := models.DNSEntry{"10.0.0.2", "", "myhost", "badnet"}
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	t.Run("EmptyNetwork", func(t *testing.T) {
		//this can't actually happen as change.Network is populated if is blank
		change := models.DNSEntry{"10.0.0.2", "", "myhost", ""}
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
		change := models.DNSEntry{"10.0.256.1", "", "myhost", "skynet"}
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'ip' tag")
	})
	t.Run("EmptyName", func(t *testing.T) {
		//this can't actually happen as change.Name is populated if is blank
		change := models.DNSEntry{"10.0.0.2", "", "", "skynet"}
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'required' tag")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		name := ""
		for i := 1; i < 194; i++ {
			name = name + "a"
		}
		change := models.DNSEntry{"10.0.0.2", "", name, "skynet"}
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("NameUnique", func(t *testing.T) {
		change := models.DNSEntry{"10.0.0.2", "", "myhost", "wirecat"}
		CreateDNS(entry)
		CreateDNS(change)
		err := logic.ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'name_unique' tag")
		//cleanup
		err = logic.DeleteDNS("myhost", "wirecat")
		assert.Nil(t, err)
	})

}
func TestValidateDNSCreate(t *testing.T) {
	database.InitializeDatabase()
	_ = logic.DeleteDNS("mynode", "skynet")
	t.Run("NoNetwork", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "", "myhost", "badnet"}
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
		entry := models.DNSEntry{"10.0.256.1", "", "myhost", "skynet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'ip' tag")
	})
	t.Run("EmptyName", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "", "", "skynet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'required' tag")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		name := ""
		for i := 1; i < 194; i++ {
			name = name + "a"
		}
		entry := models.DNSEntry{"10.0.0.2", "", name, "skynet"}
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("NameUnique", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "", "myhost", "skynet"}
		_, _ = CreateDNS(entry)
		err := logic.ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'name_unique' tag")
	})
}

func deleteAllDNS(t *testing.T) {
	dns, err := logic.GetAllDNS()
	assert.Nil(t, err)
	for _, record := range dns {
		err := logic.DeleteDNS(record.Name, record.Network)
		assert.Nil(t, err)
	}
}
