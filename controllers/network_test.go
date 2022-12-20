package controller

import (
	"os"
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

type NetworkValidationTestCase struct {
	testname   string
	network    models.Network
	errMessage string
}

func TestCreateNetwork(t *testing.T) {
	initialize()
	deleteAllNetworks()

	var network models.Network
	network.NetID = "skynet"
	network.AddressRange = "10.0.0.1/24"
	// if tests break - check here (removed displayname)
	//network.DisplayName = "mynetwork"

	_, err := logic.CreateNetwork(network)
	assert.Nil(t, err)
}
func TestGetNetwork(t *testing.T) {
	initialize()
	createNet()

	t.Run("GetExistingNetwork", func(t *testing.T) {
		network, err := logic.GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "skynet", network.NetID)
	})
	t.Run("GetNonExistantNetwork", func(t *testing.T) {
		network, err := logic.GetNetwork("doesnotexist")
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, "", network.NetID)
	})
}

func TestDeleteNetwork(t *testing.T) {
	initialize()
	createNet()
	//create nodes
	t.Run("NetworkwithNodes", func(t *testing.T) {
	})
	t.Run("DeleteExistingNetwork", func(t *testing.T) {
		err := logic.DeleteNetwork("skynet")
		assert.Nil(t, err)
	})
	t.Run("NonExistantNetwork", func(t *testing.T) {
		err := logic.DeleteNetwork("skynet")
		assert.Nil(t, err)
	})
}

func TestCreateKey(t *testing.T) {
	initialize()
	createNet()
	keys, _ := logic.GetKeys("skynet")
	for _, key := range keys {
		logic.DeleteKey(key.Name, "skynet")
	}
	var accesskey models.AccessKey
	var network models.Network
	network.NetID = "skynet"
	t.Run("NameTooLong", func(t *testing.T) {
		network, err := logic.GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "ThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfailThisisareallylongkeynamethatwillfail"
		_, err = logic.CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Name' failed on the 'max' tag")
	})
	t.Run("BlankName", func(t *testing.T) {
		network, err := logic.GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = ""
		key, err := logic.CreateAccessKey(accesskey, network)
		assert.Nil(t, err)
		assert.NotEqual(t, "", key.Name)
	})
	t.Run("InvalidValue", func(t *testing.T) {
		network, err := logic.GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Value = "bad-value"
		_, err = logic.CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Value' failed on the 'alphanum' tag")
	})
	t.Run("BlankValue", func(t *testing.T) {
		network, err := logic.GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "mykey"
		accesskey.Value = ""
		key, err := logic.CreateAccessKey(accesskey, network)
		assert.Nil(t, err)
		assert.NotEqual(t, "", key.Value)
		assert.Equal(t, accesskey.Name, key.Name)
	})
	t.Run("ValueTooLong", func(t *testing.T) {
		network, err := logic.GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "keyname"
		accesskey.Value = "AccessKeyValuethatistoolong"
		_, err = logic.CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "Field validation for 'Value' failed on the 'max' tag")
	})
	t.Run("BlankUses", func(t *testing.T) {
		network, err := logic.GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Uses = 0
		accesskey.Value = ""
		key, err := logic.CreateAccessKey(accesskey, network)
		assert.Nil(t, err)
		assert.Equal(t, 1, key.Uses)
	})
	t.Run("DuplicateKey", func(t *testing.T) {
		network, err := logic.GetNetwork("skynet")
		assert.Nil(t, err)
		accesskey.Name = "mykey"
		_, err = logic.CreateAccessKey(accesskey, network)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "duplicate AccessKey Name")
	})
}

func TestGetKeys(t *testing.T) {
	initialize()
	deleteAllNetworks()
	createNet()
	network, err := logic.GetNetwork("skynet")
	assert.Nil(t, err)
	var key models.AccessKey
	key.Name = "mykey"
	_, err = logic.CreateAccessKey(key, network)
	assert.Nil(t, err)
	t.Run("KeyExists", func(t *testing.T) {
		keys, err := logic.GetKeys(network.NetID)
		assert.Nil(t, err)
		assert.NotEqual(t, models.AccessKey{}, keys)
	})
	t.Run("NonExistantKey", func(t *testing.T) {
		err := logic.DeleteKey("mykey", "skynet")
		assert.Nil(t, err)
		keys, err := logic.GetKeys(network.NetID)
		assert.Nil(t, err)
		assert.Equal(t, []models.AccessKey(nil), keys)
	})
}
func TestDeleteKey(t *testing.T) {
	initialize()
	createNet()
	network, err := logic.GetNetwork("skynet")
	assert.Nil(t, err)
	var key models.AccessKey
	key.Name = "mykey"
	_, err = logic.CreateAccessKey(key, network)
	assert.Nil(t, err)
	t.Run("ExistingKey", func(t *testing.T) {
		err := logic.DeleteKey("mykey", "skynet")
		assert.Nil(t, err)
	})
	t.Run("NonExistantKey", func(t *testing.T) {
		err := logic.DeleteKey("mykey", "skynet")
		assert.NotNil(t, err)
		assert.Equal(t, "key mykey does not exist", err.Error())
	})
}

func TestSecurityCheck(t *testing.T) {
	//these seem to work but not sure it the tests are really testing the functionality

	initialize()
	os.Setenv("MASTER_KEY", "secretkey")
	t.Run("NoNetwork", func(t *testing.T) {
		networks, username, err := logic.UserPermissions(false, "", "Bearer secretkey")
		assert.Nil(t, err)
		t.Log(networks, username)
	})
	t.Run("WithNetwork", func(t *testing.T) {
		networks, username, err := logic.UserPermissions(false, "skynet", "Bearer secretkey")
		assert.Nil(t, err)
		t.Log(networks, username)
	})
	t.Run("BadNet", func(t *testing.T) {
		t.Skip()
		networks, username, err := logic.UserPermissions(false, "badnet", "Bearer secretkey")
		assert.NotNil(t, err)
		t.Log(err)
		t.Log(networks, username)
	})
	t.Run("BadToken", func(t *testing.T) {
		networks, username, err := logic.UserPermissions(false, "skynet", "Bearer badkey")
		assert.NotNil(t, err)
		t.Log(err)
		t.Log(networks, username)
	})
}

func TestValidateNetwork(t *testing.T) {
	//t.Skip()
	//This functions is not called by anyone
	//it panics as validation function 'display_name_valid' is not defined
	initialize()
	//yes := true
	//no := false
	//deleteNet(t)

	//DeleteNetworks
	cases := []NetworkValidationTestCase{
		{
			testname: "InvalidAddress",
			network: models.Network{
				NetID:        "skynet",
				AddressRange: "10.0.0.256",
			},
			errMessage: "Field validation for 'AddressRange' failed on the 'cidrv4' tag",
		},
		{
			testname: "InvalidAddress6",
			network: models.Network{
				NetID:         "skynet1",
				AddressRange6: "2607::ffff/130",
			},
			errMessage: "Field validation for 'AddressRange6' failed on the 'cidrv6' tag",
		},
		{
			testname: "InvalidNetID",
			network: models.Network{
				NetID: "with spaces",
			},
			errMessage: "Field validation for 'NetID' failed on the 'netid_valid' tag",
		},
		{
			testname: "NetIDTooLong",
			network: models.Network{
				NetID: "LongNetIDName",
			},
			errMessage: "Field validation for 'NetID' failed on the 'max' tag",
		},
		{
			testname: "ListenPortTooLow",
			network: models.Network{
				NetID:             "skynet",
				DefaultListenPort: 1023,
			},
			errMessage: "Field validation for 'DefaultListenPort' failed on the 'min' tag",
		},
		{
			testname: "ListenPortTooHigh",
			network: models.Network{
				NetID:             "skynet",
				DefaultListenPort: 65536,
			},
			errMessage: "Field validation for 'DefaultListenPort' failed on the 'max' tag",
		},
		{
			testname: "KeepAliveTooBig",
			network: models.Network{
				NetID:            "skynet",
				DefaultKeepalive: 1010,
			},
			errMessage: "Field validation for 'DefaultKeepalive' failed on the 'max' tag",
		},
		{
			testname: "InvalidLocalRange",
			network: models.Network{
				NetID:      "skynet",
				LocalRange: "192.168.0.1",
			},
			errMessage: "Field validation for 'LocalRange' failed on the 'cidr' tag",
		},
	}
	for _, tc := range cases {
		t.Run(tc.testname, func(t *testing.T) {
			t.Log(tc.testname)
			network := models.Network(tc.network)
			network.SetDefaults()
			err := logic.ValidateNetwork(&network, false)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tc.errMessage) // test passes if err.Error() contains the expected errMessage.
		})
	}
}

func TestIpv6Network(t *testing.T) {
	//these seem to work but not sure it the tests are really testing the functionality

	initialize()
	os.Setenv("MASTER_KEY", "secretkey")
	deleteAllNetworks()
	createNet()
	createNetDualStack()
	network, err := logic.GetNetwork("skynet6")
	t.Run("Test Network Create IPv6", func(t *testing.T) {
		assert.Nil(t, err)
		assert.Equal(t, network.AddressRange6, "fde6:be04:fa5e:d076::/64")
	})
	node1 := models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Name: "testnode", Endpoint: "10.0.0.50", MacAddress: "01:02:03:04:05:06", Password: "password", Network: "skynet6", OS: "linux"}
	nodeErr := logic.CreateNode(&node1)
	t.Run("Test node on network IPv6", func(t *testing.T) {
		assert.Nil(t, nodeErr)
		assert.Equal(t, "fde6:be04:fa5e:d076::1", node1.Address6)
	})
}

func deleteAllNetworks() {
	deleteAllNodes()
	nets, _ := logic.GetNetworks()
	for _, net := range nets {
		logic.DeleteNetwork(net.NetID)
	}
}

func initialize() {
	database.InitializeDatabase()
	createAdminUser()
}

func createAdminUser() {
	logic.CreateAdmin(&models.User{
		UserName: "admin",
		Password: "password",
		IsAdmin:  true,
		Networks: []string{},
		Groups:   []string{},
	})
}

func createNet() {
	var network models.Network
	network.NetID = "skynet"
	network.AddressRange = "10.0.0.1/24"
	_, err := logic.GetNetwork("skynet")
	if err != nil {
		logic.CreateNetwork(network)
	}
}

func createNetDualStack() {
	var network models.Network
	network.NetID = "skynet6"
	network.AddressRange = "10.1.2.0/24"
	network.AddressRange6 = "fde6:be04:fa5e:d076::/64"
	network.IsIPv4 = "yes"
	network.IsIPv6 = "yes"
	_, err := logic.GetNetwork("skynet6")
	if err != nil {
		logic.CreateNetwork(network)
	}
}
