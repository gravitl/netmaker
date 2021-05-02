package controller

import (
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

type NetworkValidationTestCase struct {
	testname   string
	network    models.Network
	errMessage string
}

func deleteNet() {
	_, err := GetNetwork("skynet")
	if err == nil {
		_, _ = DeleteNetwork("skynet")
	}
}

func createNet() {
	var network models.Network
	network.NetID = "skynet"
	network.AddressRange = "10.0.0.1/24"
	network.DisplayName = "mynetwork"
	_, err := GetNetwork("skynet")
	if err != nil {
		CreateNetwork(network)
	}
}

func TestGetNetworks(t *testing.T) {
	//calls functions.ListNetworks --- nothing to be done
}
func TestCreateNetwork(t *testing.T) {
	deleteNet()
	var network models.Network
	network.NetID = "skynet"
	network.AddressRange = "10.0.0.1/24"
	network.DisplayName = "mynetwork"
	err := CreateNetwork(network)
	assert.Nil(t, err)
}
func TestGetDeleteNetwork(t *testing.T) {
	createNet()
	//create nodes
	t.Run("NetworkwithNodes", func(t *testing.T) {
	})
	t.Run("GetExistingNetwork", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "skynet", network.NetID)
	})
	t.Run("DeleteExistingNetwork", func(t *testing.T) {
		result, err := DeleteNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, int64(1), result.DeletedCount)
		t.Log(result.DeletedCount)
	})
	t.Run("GetNonExistantNetwork", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.NotNil(t, err)
		assert.Equal(t, "mongo: no documents in result", err.Error())
		assert.Equal(t, "", network.NetID)
	})
	t.Run("NonExistantNetwork", func(t *testing.T) {
		result, err := DeleteNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, int64(0), result.DeletedCount)
		t.Log(result.DeletedCount)
	})
}
func TestGetNetwork(t *testing.T) {
	createNet()
	t.Run("NoNetwork", func(t *testing.T) {
		network, err := GetNetwork("badnet")
		assert.NotNil(t, err)
		assert.Equal(t, "mongo: no documents in result", err.Error())
		assert.Equal(t, models.Network{}, network)
	})
	t.Run("Valid", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "skynet", network.NetID)
	})
}
func TestUpdateNetwork(t *testing.T) {
}

func TestKeyUpdate(t *testing.T) {
	createNet()
	existing, err := GetNetwork("skynet")
	assert.Nil(t, err)
	time.Sleep(time.Second * 1)
	network, err := KeyUpdate("skynet")
	assert.Nil(t, err)
	network, err = GetNetwork("skynet")
	assert.Nil(t, err)
	assert.Greater(t, network.KeyUpdateTimeStamp, existing.KeyUpdateTimeStamp)
}

func TestCreateKey(t *testing.T) {
	createNet()
	var accesskey models.AccessKey
	var network models.Network
	network.NetID = "skynet"
	t.Run("InvalidName", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "bad-name"
		_, err = CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'alphanum' tag")
	})
	t.Run("NameTooLong", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "Thisisareallylongkeynamethatwillfail"
		_, err = CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("BlankName", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = ""
		key, err := CreateAccessKey(accesskey, network)
		assert.Nil(t, err)
		assert.NotEqual(t, "", key.Name)
	})
	t.Run("InvalidValue", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Value = "bad-value"
		_, err = CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Value' failed on the 'alphanum' tag")
	})
	t.Run("BlankValue", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "mykey"
		accesskey.Value = ""
		key, err := CreateAccessKey(accesskey, network)
		assert.Nil(t, err)
		assert.NotEqual(t, "", key.Value)
		assert.Equal(t, accesskey.Name, key.Name)
	})
	t.Run("ValueTooLong", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "keyname"
		accesskey.Value = "AccessKeyValuethatistoolong"
		_, err = CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Value' failed on the 'max' tag")
	})
	t.Run("BlankUses", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Uses = 0
		accesskey.Value = ""
		key, err := CreateAccessKey(accesskey, network)
		assert.Nil(t, err)
		assert.Equal(t, 1, key.Uses)
	})
	t.Run("DuplicateKey", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "mykey"
		_, err = CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.Equal(t, "Duplicate AccessKey Name", err.Error())
	})
}
func TestGetKeys(t *testing.T) {
	deleteNet()
	createNet()
	network, err := GetNetwork("skynet")
	assert.Nil(t, err)
	var key models.AccessKey
	key.Name = "mykey"
	_, err = CreateAccessKey(key, network)
	assert.Nil(t, err)
	t.Run("KeyExists", func(t *testing.T) {
		keys, err := GetKeys(network.NetID)
		assert.Nil(t, err)
		assert.NotEqual(t, models.AccessKey{}, keys)
	})
	t.Run("NonExistantKey", func(t *testing.T) {
		err := DeleteKey("mykey", "skynet")
		assert.Nil(t, err)
		keys, err := GetKeys(network.NetID)
		assert.Nil(t, err)
		assert.Equal(t, []models.AccessKey(nil), keys)
	})
}
func TestDeleteKey(t *testing.T) {
	createNet()
	network, err := GetNetwork("skynet")
	assert.Nil(t, err)
	var key models.AccessKey
	key.Name = "mykey"
	_, err = CreateAccessKey(key, network)
	assert.Nil(t, err)
	t.Run("ExistingKey", func(t *testing.T) {
		err := DeleteKey("mykey", "skynet")
		assert.Nil(t, err)
	})
	t.Run("NonExistantKey", func(t *testing.T) {
		err := DeleteKey("mykey", "skynet")
		assert.NotNil(t, err)
		assert.Equal(t, "key mykey does not exist", err.Error())
	})
}
func TestSecurityCheck(t *testing.T) {
	t.Run("NoNetwork", func(t *testing.T) {
		err := SecurityCheck("", "Bearer secretkey")
		assert.Nil(t, err)
	})
	t.Run("WithNetwork", func(t *testing.T) {
		err := SecurityCheck("skynet", "Bearer secretkey")
		assert.Nil(t, err)
	})
	t.Run("BadNet", func(t *testing.T) {
		err := SecurityCheck("badnet", "Bearer secretkey")
		assert.NotNil(t, err)
		t.Log(err)
	})
	t.Run("BadToken", func(t *testing.T) {
		err := SecurityCheck("skynet", "Bearer badkey")
		assert.NotNil(t, err)
		t.Log(err)
	})
}
func TestValidateNetworkUpdate(t *testing.T) {
}
func TestValidateNetworkCreate(t *testing.T) {
	yes := true
	no := false
	deleteNet()
	//DeleteNetworks
	cases := []NetworkValidationTestCase{
		NetworkValidationTestCase{
			testname: "InvalidAddress",
			network: models.Network{
				AddressRange: "10.0.0.256",
				NetID:        "skynet",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'AddressRange' failed on the 'cidr' tag",
		},
		NetworkValidationTestCase{
			testname: "BadDisplayName",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				DisplayName:  "skynet*",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'DisplayName' failed on the 'alphanum' tag",
		},
		NetworkValidationTestCase{
			testname: "DisplayNameTooLong",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				DisplayName:  "Thisisareallylongdisplaynamethatistoolong",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'DisplayName' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "DisplayNameTooShort",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				DisplayName:  "1",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'DisplayName' failed on the 'min' tag",
		},
		NetworkValidationTestCase{
			testname: "NetIDMissing",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'NetID' failed on the 'required' tag",
		},
		NetworkValidationTestCase{
			testname: "InvalidNetID",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "contains spaces",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'NetID' failed on the 'alphanum' tag",
		},
		NetworkValidationTestCase{
			testname: "NetIDTooShort",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'NetID' failed on the 'required' tag",
		},
		NetworkValidationTestCase{
			testname: "NetIDTooLong",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "LongNetIDName",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'NetID' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "ListenPortTooLow",
			network: models.Network{
				AddressRange:      "10.0.0.1/24",
				NetID:             "skynet",
				DefaultListenPort: 1023,
				IsDualStack:       &no,
			},
			errMessage: "Field validation for 'DefaultListenPort' failed on the 'min' tag",
		},
		NetworkValidationTestCase{
			testname: "ListenPortTooHigh",
			network: models.Network{
				AddressRange:      "10.0.0.1/24",
				NetID:             "skynet",
				DefaultListenPort: 65536,
				IsDualStack:       &no,
			},
			errMessage: "Field validation for 'DefaultListenPort' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "KeepAliveTooBig",
			network: models.Network{
				AddressRange:     "10.0.0.1/24",
				NetID:            "skynet",
				DefaultKeepalive: 1010,
				IsDualStack:      &no,
			},
			errMessage: "Field validation for 'DefaultKeepalive' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "InvalidLocalRange",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				LocalRange:   "192.168.0.1",
				IsDualStack:  &no,
			},
			errMessage: "Field validation for 'LocalRange' failed on the 'cidr' tag",
		},
		NetworkValidationTestCase{
			testname: "DualStackWithoutIPv6",
			network: models.Network{
				AddressRange: "10.0.0.1/24",
				NetID:        "skynet",
				IsDualStack:  &yes,
			},
			errMessage: "Field validation for 'AddressRange6' failed on the 'addressrange6_valid' tag",
		},
		NetworkValidationTestCase{
			testname: "CheckInIntervalTooBig",
			network: models.Network{
				AddressRange:           "10.0.0.1/24",
				NetID:                  "skynet",
				IsDualStack:            &no,
				DefaultCheckInInterval: 100001,
			},
			errMessage: "Field validation for 'DefaultCheckInInterval' failed on the 'max' tag",
		},
		NetworkValidationTestCase{
			testname: "CheckInIntervalTooSmall",
			network: models.Network{
				AddressRange:           "10.0.0.1/24",
				NetID:                  "skynet",
				IsDualStack:            &no,
				DefaultCheckInInterval: 1,
			},
			errMessage: "Field validation for 'DefaultCheckInInterval' failed on the 'min' tag",
		},
	}
	for _, tc := range cases {
		t.Run(tc.testname, func(t *testing.T) {
			err := ValidateNetworkCreate(tc.network)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tc.errMessage)
		})
	}
	t.Run("DuplicateNetID", func(t *testing.T) {
		var net1, net2 models.Network
		net1.NetID = "skynet"
		net1.AddressRange = "10.0.0.1/24"
		net1.DisplayName = "mynetwork"
		net2.NetID = "skynet"
		net2.AddressRange = "10.0.1.1/24"
		net2.IsDualStack = &no

		err := CreateNetwork(net1)
		assert.Nil(t, err)
		err = ValidateNetworkCreate(net2)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'NetID' failed on the 'netid_valid' tag")
	})
	t.Run("DuplicateDisplayName", func(t *testing.T) {
		var network models.Network
		network.NetID = "wirecat"
		network.AddressRange = "10.0.100.1/24"
		network.IsDualStack = &no
		network.DisplayName = "mynetwork"
		err := ValidateNetworkCreate(network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'DisplayName' failed on the 'displayname_unique' tag")
	})

}
