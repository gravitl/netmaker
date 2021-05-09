package controller

import (
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestGetNodeDNS(t *testing.T) {
	dns, err := GetNodeDNS("skynet")
	assert.Nil(t, err)
	t.Log(dns)
}
func TestGetCustomDNS(t *testing.T) {
	dns, err := GetCustomDNS("skynet")
	assert.Nil(t, err)
	t.Log(dns)
}
func TestGetDNSEntryNum(t *testing.T) {
	num, err := GetDNSEntryNum("myhost", "skynet")
	assert.Nil(t, err)
	t.Log(num)
}
func TestGetDNS(t *testing.T) {
	dns, err := GetDNS("skynet")
	assert.Nil(t, err)
	t.Log(dns)
}
func TestCreateDNS(t *testing.T) {
	deleteNet(t)
	createNet()
	//dns, err := GetDNS("skynet")
	//assert.Nil(t, err)
	//for _, entry := range dns {
	//	_, _ = DeleteDNS(entry.Name, "skynet")
	//}
	entry := models.DNSEntry{"10.0.0.2", "myhost", "skynet"}
	err := ValidateDNSCreate(entry)
	assert.Nil(t, err)
	if err != nil {
		return
	}
	dns, err := CreateDNS(entry)
	assert.Nil(t, err)
	t.Log(dns)
}
func TestGetDNSEntry(t *testing.T) {
	entry, err := GetDNSEntry("myhost", "skynet")
	assert.Nil(t, err)
	t.Log(entry)
}
func TestUpdateDNS(t *testing.T) {
}
func TestDeleteDNS(t *testing.T) {
	t.Run("EntryExists", func(t *testing.T) {
		success, err := DeleteDNS("myhost", "skynet")
		assert.Nil(t, err)
		assert.True(t, success)
	})
	t.Run("NoEntry", func(t *testing.T) {
		success, err := DeleteDNS("myhost", "skynet")
		assert.Nil(t, err)
		assert.False(t, success)
	})

}

func TestValidateDNSUpdate(t *testing.T) {
	entry := models.DNSEntry{"10.0.0.2", "myhost", "skynet"}
	_, _ = DeleteDNS("mynode", "skynet")
	t.Run("BadNetwork", func(t *testing.T) {
		change := models.DNSEntry{"10.0.0.2", "myhost", "badnet"}
		err := ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	t.Run("EmptyNetwork", func(t *testing.T) {
		//this can't actually happen as change.Network is populated if is blank
		change := models.DNSEntry{"10.0.0.2", "myhost", ""}
		err := ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	t.Run("EmptyAddress", func(t *testing.T) {
		//this can't actually happen as change.Address is populated if is blank
		change := models.DNSEntry{"", "myhost", "skynet"}
		err := ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'required' tag")
	})
	t.Run("BadAddress", func(t *testing.T) {
		change := models.DNSEntry{"10.0.256.1", "myhost", "skynet"}
		err := ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'ip' tag")
	})
	t.Run("BadName", func(t *testing.T) {
		change := models.DNSEntry{"10.0.0.2", "myhostr*", "skynet"}
		err := ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'alphanum' tag")
	})
	t.Run("EmptyName", func(t *testing.T) {
		//this can't actually happen as change.Name is populated if is blank
		change := models.DNSEntry{"10.0.0.2", "", "skynet"}
		err := ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'required' tag")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		name := ""
		for i := 1; i < 122; i++ {
			name = name + "a"
		}
		change := models.DNSEntry{"10.0.0.2", name, "skynet"}
		err := ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("NameUnique", func(t *testing.T) {
		change := models.DNSEntry{"10.0.0.2", "myhost", "wirecat"}
		_, _ = CreateDNS(entry)
		_, _ = CreateDNS(change)
		err := ValidateDNSUpdate(change, entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'name_unique' tag")
	})

}
func TestValidateDNSCreate(t *testing.T) {
	_, _ = DeleteDNS("mynode", "skynet")
	t.Run("NoNetwork", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "myhost", "badnet"}
		err := ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Network' failed on the 'network_exists' tag")
	})
	t.Run("EmptyAddress", func(t *testing.T) {
		entry := models.DNSEntry{"", "myhost", "skynet"}
		err := ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'required' tag")
	})
	t.Run("BadAddress", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.256.1", "myhost", "skynet"}
		err := ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Address' failed on the 'ip' tag")
	})
	t.Run("BadName", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "myhostr*", "skynet"}
		err := ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'alphanum' tag")
	})
	t.Run("EmptyName", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "", "skynet"}
		err := ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'required' tag")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		name := ""
		for i := 1; i < 122; i++ {
			name = name + "a"
		}
		entry := models.DNSEntry{"10.0.0.2", name, "skynet"}
		err := ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("NameUnique", func(t *testing.T) {
		entry := models.DNSEntry{"10.0.0.2", "myhost", "skynet"}
		_, _ = CreateDNS(entry)
		err := ValidateDNSCreate(entry)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'name_unique' tag")
	})
}
