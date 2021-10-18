package controller

import (
	"testing"
	"time"

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
	database.InitializeDatabase()
	deleteAllNetworks()

	var network models.Network
	network.NetID = "skynet"
	network.AddressRange = "10.0.0.1/24"
	network.DisplayName = "mynetwork"

	err := CreateNetwork(network)
	assert.Nil(t, err)
}
func TestGetNetwork(t *testing.T) {
	database.InitializeDatabase()
	createNet()

	t.Run("GetExistingNetwork", func(t *testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "skynet", network.NetID)
	})
	t.Run("GetNonExistantNetwork", func(t *testing.T) {
		network, err := GetNetwork("doesnotexist")
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, "", network.NetID)
	})
}

func TestDeleteNetwork(t *testing.T) {
	database.InitializeDatabase()
	createNet()
	//create nodes
	t.Run("NetworkwithNodes", func(t *testing.T) {
	})
	t.Run("DeleteExistingNetwork", func(t *testing.T) {
		err := DeleteNetwork("skynet")
		assert.Nil(t, err)
	})
	t.Run("NonExistantNetwork", func(t *testing.T) {
		err := DeleteNetwork("skynet")
		assert.Nil(t, err)
	})
}

func TestKeyUpdate(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("InvalidNetwork", func(*testing.T) {
		_, err := KeyUpdate("badnet")
		assert.Nil(t, err)
		nodes, err := logic.GetNetworkNodes("badnet")
		assert.Nil(t, err)
		assert.Equal(t, 0, len(nodes))
	})
	t.Run("ValidNetwork", func(*testing.T) {
		createTestNode()
		_, err := KeyUpdate("skynet")
		assert.Nil(t, err)
		nodes, err := logic.GetNetworkNodes("skynet")
		assert.Nil(t, err)
		for _, node := range nodes {
			assert.Equal(t, models.NODE_UPDATE_KEY, node.Action)
		}
	})
}

func TestCreateKey(t *testing.T) {
	database.InitializeDatabase()
	createNet()
	keys, _ := GetKeys("skynet")
	for _, key := range keys {
		DeleteKey(key.Name, "skynet")
	}
	var accesskey models.AccessKey
	var network models.Network
	network.NetID = "skynet"
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
		assert.EqualError(t, err, "duplicate AccessKey Name")
	})
}

func TestGetKeys(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
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
	database.InitializeDatabase()
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
	//these seem to work but not sure it the tests are really testing the functionality

	database.InitializeDatabase()
	t.Run("NoNetwork", func(t *testing.T) {
		err, networks, username := SecurityCheck(false, "", "Bearer secretkey")
		assert.Nil(t, err)
		t.Log(networks, username)
	})
	t.Run("WithNetwork", func(t *testing.T) {
		err, networks, username := SecurityCheck(false, "skynet", "Bearer secretkey")
		assert.Nil(t, err)
		t.Log(networks, username)
	})
	t.Run("BadNet", func(t *testing.T) {
		t.Skip()
		err, networks, username := SecurityCheck(false, "badnet", "Bearer secretkey")
		assert.NotNil(t, err)
		t.Log(err)
		t.Log(networks, username)
	})
	t.Run("BadToken", func(t *testing.T) {
		err, networks, username := SecurityCheck(false, "skynet", "Bearer badkey")
		assert.NotNil(t, err)
		t.Log(err)
		t.Log(networks, username)
	})
}

func TestGetSignupToken(t *testing.T) {
	database.InitializeDatabase()
	key, err := GetSignupToken("skynet")
	assert.Nil(t, err)
	assert.NotNil(t, key.AccessString)
}

func TestAlertNetwork(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("ValidNetwork", func(*testing.T) {
		now, err := GetNetwork("skynet")
		time.Sleep(1 * time.Second)
		assert.Nil(t, err)
		err = AlertNetwork("skynet")
		assert.Nil(t, err)
		updated, err := GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Greater(t, updated.NodesLastModified, now.NodesLastModified)
		assert.Greater(t, updated.NetworkLastModified, now.NetworkLastModified)
	})
	t.Run("BadNetwork", func(*testing.T) {
		err := AlertNetwork("badnet")
		assert.EqualError(t, err, "no result found")
	})

}

func TestNetworkUpdate(t *testing.T) {
	//var newNet models.Network
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("SimpleChange", func(*testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		newnet, err := GetNetwork("skynet")
		assert.Nil(t, err)
		newnet.DisplayName = "HelloWorld"
		rangeupdate, localupdate, err := network.Update(&newnet)
		assert.Nil(t, err)
		assert.False(t, rangeupdate)
		assert.False(t, localupdate)
		net, err := GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "HelloWorld", net.DisplayName)
	})
	t.Run("RangeChange", func(*testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		newnet, err := GetNetwork("skynet")
		assert.Nil(t, err)
		newnet.AddressRange = "10.100.100.0/24"
		rangeupdate, localupdate, err := network.Update(&newnet)
		assert.Nil(t, err)
		assert.True(t, rangeupdate)
		assert.False(t, localupdate)
		net, err := GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "10.100.100.0/24", net.AddressRange)
	})
	t.Run("LocalChange", func(*testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		newnet, err := GetNetwork("skynet")
		assert.Nil(t, err)
		newnet.LocalRange = "192.168.0.0/24"
		rangeupdate, localupdate, err := network.Update(&newnet)
		assert.Nil(t, err)
		assert.False(t, rangeupdate)
		assert.True(t, localupdate)
		net, err := GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "192.168.0.0/24", net.LocalRange)
	})
	t.Run("NetID", func(*testing.T) {
		network, err := GetNetwork("skynet")
		assert.Nil(t, err)
		newnet, err := GetNetwork("skynet")
		assert.Nil(t, err)
		newnet.NetID = "bad"
		rangeupdate, localupdate, err := network.Update(&newnet)
		assert.EqualError(t, err, "failed to update network bad, cannot change netid.")
		assert.False(t, rangeupdate)
		assert.False(t, localupdate)
		net, err := GetNetwork("skynet")
		assert.Nil(t, err)
		assert.Equal(t, "skynet", net.NetID)
	})

}

func deleteAllNetworks() {
	deleteAllNodes()
	nets, _ := logic.GetNetworks()
	for _, net := range nets {
		DeleteNetwork(net.NetID)
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

func getNet() models.Network {
	network, _ := GetNetwork("skynet")
	return network
}
