//go:build integration
// +build integration

package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gravitl/netmaker/cli/config"
	"github.com/gravitl/netmaker/cli/functions"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/stretchr/testify/assert"
)

func DBInit() {
	database.InitializeDatabase()
	database.DeleteAllRecords(database.USERS_TABLE_NAME)
	database.DeleteAllRecords(database.NETWORKS_TABLE_NAME)
	database.DeleteAllRecords(database.NETWORK_USER_TABLE_NAME)
	database.DeleteAllRecords(database.ENROLLMENT_KEYS_TABLE_NAME)
	// TODO rest
}

func TestHasNetworksAccessAPI(t *testing.T) {
	// setup / teardown (TODO extract)
	DBInit()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Wait()
		defer database.CloseDB()
	}()
	var err error
	port := servercfg.GetAPIPort()
	userPass := "bar123"
	user := &models.User{
		UserName: "foo",
		Password: userPass,
		// TODO should be handled in fixtures?
		Networks: []string{"network-1"},
		IsAdmin:  false,
		Groups:   nil,
	}
	err = logic.CreateUser(user)
	if err != nil {
		t.Error("Error creating a user ", err)
	}
	// create configs
	userConfig := config.Context{
		Endpoint: "http://localhost:" + port,
		Username: user.UserName,
		Password: userPass,
	}
	adminConfig := userConfig
	adminConfig.MasterKey = "foo123"
	adminConfigBad := userConfig
	adminConfigBad.MasterKey = "wrongpass"
	adminConfigBad.Password = "wrongpass"
	// add configs
	config.SetContext("user-ctx-1", userConfig)
	config.SetContext("admin-ctx-1", adminConfig)
	config.SetContext("admin-ctx-2", adminConfigBad)
	// set the active config
	config.SetCurrentContext("user-ctx-1")
	t.Setenv("MASTER_KEY", adminConfig.MasterKey)

	// fixtures
	n1 := models.Network{
		AddressRange:        "10.101.0.0/16",
		NetID:               "network-1",
		NodesLastModified:   1685013908,
		NetworkLastModified: 1684474527,
		DefaultInterface:    "nm-netmaker",
		DefaultListenPort:   51821,
		NodeLimit:           999999999,
		DefaultPostDown:     "",
		DefaultKeepalive:    20,
		AllowManualSignUp:   "no",
		IsIPv4:              "yes",
		IsIPv6:              "no",
		DefaultUDPHolePunch: "no",
		DefaultMTU:          1280,
		DefaultACL:          "yes",
		ProSettings:         nil,
	}
	_, err = logic.CreateNetwork(n1)
	if err != nil {
		t.Error("Error creating a network ", err)
	}
	// copy
	n2 := n1
	n2.NetID = "network-2"
	_, err = logic.CreateNetwork(n2)
	if err != nil {
		t.Error("Error creating a network ", err)
	}
	k1, _ := logic.CreateEnrollmentKey(0, time.Time{}, []string{n1.NetID}, nil, true)
	if err = logic.Tokenize(k1, servercfg.GetAPIHost()); err != nil {
		t.Error("failed to get token values for keys:", err)
	}
	_, _ = logic.CreateEnrollmentKey(0, time.Time{}, []string{n2.NetID}, nil, true)
	_, _ = logic.CreateEnrollmentKey(0, time.Time{}, []string{n1.NetID, n2.NetID}, nil, true)

	go controller.HandleRESTRequests(&wg, ctx)
	// TODO make sure that HTTP is up
	time.Sleep(1 * time.Second)

	t.Run("normal user", func(t *testing.T) {
		keys := *functions.GetEnrollmentKeys()
		assert.Len(t, keys, 1, "1 key expected")
		assert.Len(t, keys[0].Networks, 1, "Key with 1 network expected")
		assert.Equal(t, keys[0].Networks[0], n1.NetID, "Network ID matches")
		assert.Equal(t, keys[0].Token, k1.Token, "Token matches")
	})

	t.Run("masteradmin", func(t *testing.T) {
		config.SetCurrentContext("admin-ctx-1")
		keys := *functions.GetEnrollmentKeys()
		assert.Len(t, keys, 3, "3 keys expected")
	})

	// TODO assert no access
	t.Run("incorrect masteradmin", func(t *testing.T) {
		t.Skip("Skipping until err exposed")
		config.SetCurrentContext("admin-ctx-2")
		// TODO doesnt return err
		res := *functions.GetEnrollmentKeys()
		fmt.Println(res)
		//assert.Error(t, res, "403 error")
	})
}
