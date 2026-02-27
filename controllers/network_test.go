package controller

import (
	"context"
	"os"
	"testing"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type NetworkValidationTestCase struct {
	testname   string
	network    schema.Network
	errMessage string
}

var netHost schema.Host

func TestMain(m *testing.M) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	database.InitializeDatabase()
	defer database.CloseDB()
	logic.CreateSuperAdmin(&schema.User{
		Username:       "admin",
		Password:       "password",
		PlatformRoleID: schema.SuperAdminRole,
	})
	peerUpdate := make(chan *models.Node)
	go logic.ManageZombies(context.Background())
	go func() {
		for update := range peerUpdate {
			//do nothing
			logger.Log(3, "received node update", update.Action)
		}
	}()
	os.Exit(m.Run())

}

func TestCreateNetwork(t *testing.T) {
	deleteAllNetworks()

	var network schema.Network
	network.Name = "skynet1"
	network.AddressRange = "10.10.0.1/24"
	// if tests break - check here (removed displayname)
	//network.DisplayName = "mynetwork"

	err := logic.CreateNetwork(&network)
	assert.Nil(t, err)
}
func TestGetNetwork(t *testing.T) {
	createNet()

	t.Run("GetExistingNetwork", func(t *testing.T) {
		network := &schema.Network{Name: "skynet"}
		err := network.Get(db.WithContext(context.TODO()))
		assert.Nil(t, err)
		assert.Equal(t, "skynet", network.Name)
	})
	t.Run("GetNonExistantNetwork", func(t *testing.T) {
		network := &schema.Network{Name: "doesnotexist"}
		err := network.Get(db.WithContext(context.TODO()))
		assert.EqualError(t, err, gorm.ErrRecordNotFound.Error())
		assert.Equal(t, "", network.ID)
	})
}

func TestDeleteNetwork(t *testing.T) {
	createNet()
	//create nodes
	t.Run("NetworkwithNodes", func(t *testing.T) {
	})
	t.Run("DeleteExistingNetwork", func(t *testing.T) {
		doneCh := make(chan struct{}, 1)
		err := logic.DeleteNetwork("skynet", false, doneCh)
		assert.Nil(t, err)
	})
	t.Run("NonExistentNetwork", func(t *testing.T) {
		doneCh := make(chan struct{}, 1)
		err := logic.DeleteNetwork("skynet", false, doneCh)
		assert.Nil(t, err)
	})
	createNetv1("test")
	t.Run("ForceDeleteNetwork", func(t *testing.T) {
		doneCh := make(chan struct{}, 1)
		err := logic.DeleteNetwork("test", true, doneCh)
		assert.Nil(t, err)
	})
}

func TestSecurityCheck(t *testing.T) {
	//these seem to work but not sure it the tests are really testing the functionality

	os.Setenv("MASTER_KEY", "secretkey")
	t.Run("NoNetwork", func(t *testing.T) {
		username, err := logic.UserPermissions(false, "Bearer secretkey")
		assert.Nil(t, err)
		t.Log(username)
	})

	t.Run("BadToken", func(t *testing.T) {
		username, err := logic.UserPermissions(false, "Bearer badkey")
		assert.NotNil(t, err)
		t.Log(err)
		t.Log(username)
	})
}

func TestValidateNetwork(t *testing.T) {
	//t.Skip()
	//This functions is not called by anyone
	//it panics as validation function 'display_name_valid' is not defined
	//yes := true
	//no := false
	//deleteNet(t)

	//DeleteNetworks
	cases := []NetworkValidationTestCase{
		{
			testname: "InvalidAddress",
			network: schema.Network{
				Name:         "skynet",
				AddressRange: "10.0.0.256",
			},
			errMessage: "invalid CIDR address: 10.0.0.256",
		},
		{
			testname: "InvalidAddress6",
			network: schema.Network{
				Name:          "skynet1",
				AddressRange6: "2607::ffff/130",
			},
			errMessage: "invalid CIDR address: 2607::ffff/130",
		},
		{
			testname: "InvalidNetID",
			network: schema.Network{
				Name: "with spaces",
			},
			errMessage: "invalid character(s) in network name",
		},
		{
			testname: "NetIDTooLong",
			network: schema.Network{
				Name: "LongNetIDNameForMaxCharactersTest",
			},
			errMessage: "network name cannot be longer than 32 characters",
		},
		{
			testname: "KeepAliveTooBig",
			network: schema.Network{
				Name:             "skynet",
				DefaultKeepAlive: 1010,
			},
			errMessage: "default keep alive must be less than 1000",
		},
	}
	for _, tc := range cases {
		t.Run(tc.testname, func(t *testing.T) {
			t.Log(tc.testname)
			network := tc.network
			err := logic.ValidateNetwork(&network, false)

			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tc.errMessage) // test passes if err.Error() contains the expected errMessage.
		})
	}
}

func TestIpv6Network(t *testing.T) {
	//these seem to work but not sure it the tests are really testing the functionality

	os.Setenv("MASTER_KEY", "secretkey")
	deleteAllNetworks()
	createNet()
	createNetDualStack()
	network := &schema.Network{Name: "skynet6"}
	err := network.Get(db.WithContext(context.TODO()))
	t.Run("Test Network Create IPv6", func(t *testing.T) {
		assert.Nil(t, err)
		assert.Equal(t, network.AddressRange6, "fde6:be04:fa5e:d076::/64")
	})
	node1 := createNodeWithParams("skynet6", "")
	createNetHost()
	nodeErr := logic.AssociateNodeToHost(node1, &netHost)
	t.Run("Test node on network IPv6", func(t *testing.T) {
		assert.Nil(t, nodeErr)
		assert.Equal(t, "fde6:be04:fa5e:d076::1", node1.Address6.IP.String())
	})
}

func deleteAllNetworks() {
	deleteAllNodes()
	_networks, _ := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	for _, _network := range _networks {
		_ = _network.Delete(db.WithContext(context.TODO()))
	}
}

func createNet() {
	var network schema.Network
	network.Name = "skynet"
	network.AddressRange = "10.0.0.1/24"
	err := (&schema.Network{Name: "skynet"}).Get(db.WithContext(context.TODO()))
	if err != nil {
		logic.CreateNetwork(&network)
	}
}
func createNetv1(netId string) {
	var network schema.Network
	network.Name = netId
	network.AddressRange = "100.0.0.1/24"
	err := (&schema.Network{Name: netId}).Get(db.WithContext(context.TODO()))
	if err != nil {
		logic.CreateNetwork(&network)
	}
}

func createNetDualStack() {
	var network schema.Network
	network.Name = "skynet6"
	network.AddressRange = "10.1.2.0/24"
	network.AddressRange6 = "fde6:be04:fa5e:d076::/64"
	err := (&schema.Network{Name: "skynet6"}).Get(db.WithContext(context.TODO()))
	if err != nil {
		logic.CreateNetwork(&network)
	}
}

func createNetHost() {
	k, _ := wgtypes.ParseKey("DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=")
	netHost = schema.Host{
		ID:        uuid.New(),
		PublicKey: k.PublicKey(),
		HostPass:  "password",
		OS:        "linux",
		Name:      "nethost",
	}
	_ = logic.CreateHost(&netHost)
}
