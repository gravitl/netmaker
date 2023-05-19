package functions

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

var (
	testNetwork = &models.Network{
		NetID: "not-a-network",
	}
	testExternalClient = &models.ExtClient{
		ClientID: "testExtClient",
	}
)

func TestMain(m *testing.M) {
	database.InitializeDatabase()
	defer database.CloseDB()
	logic.CreateAdmin(&models.User{
		UserName: "admin",
		Password: "password",
		IsAdmin:  true,
		Networks: []string{},
		Groups:   []string{},
	})
	peerUpdate := make(chan *models.Node)
	go logic.ManageZombies(context.Background(), peerUpdate)
	go func() {
		for update := range peerUpdate {
			//do nothing
			logger.Log(3, "received node update", update.Action)
		}
	}()
	os.Exit(m.Run())

}

func TestNetworkExists(t *testing.T) {
	database.DeleteRecord(database.NETWORKS_TABLE_NAME, testNetwork.NetID)
	exists, err := logic.NetworkExists(testNetwork.NetID)
	assert.NotNil(t, err)
	assert.False(t, exists)

	err = logic.SaveNetwork(testNetwork)
	assert.Nil(t, err)
	exists, err = logic.NetworkExists(testNetwork.NetID)
	assert.Nil(t, err)
	assert.True(t, exists)

	err = database.DeleteRecord(database.NETWORKS_TABLE_NAME, testNetwork.NetID)
	assert.Nil(t, err)
}

func TestGetAllExtClients(t *testing.T) {
	err := database.DeleteRecord(database.EXT_CLIENT_TABLE_NAME, testExternalClient.ClientID)
	assert.Nil(t, err)

	extClients, err := GetAllExtClients()
	if err == nil {
		t.Fatalf("expected error, received nil")
	}
	if len(extClients) >= 1 {
		t.Fatalf("expected no external clients, received %d", len(extClients))
	}

	extClient, err := json.Marshal(testExternalClient)
	if err != nil {
		t.Fatal(err)
	}

	err = database.Insert(testExternalClient.ClientID, string(extClient), database.EXT_CLIENT_TABLE_NAME)
	if err != nil {
		t.Fatal(err)
	}

	extClients, err = GetAllExtClients()
	if err != nil {
		t.Fatalf("expected nil, received: %s", err)
	}
	if len(extClients) < 1 {
		t.Fatalf("expected 1 external client, received %d", len(extClients))
	}

	err = database.DeleteRecord(database.EXT_CLIENT_TABLE_NAME, testExternalClient.ClientID)
	if err != nil {
		t.Fatalf("failed removing extclient: %s", err)
	}
}
