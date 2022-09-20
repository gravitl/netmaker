package functions

import (
	"encoding/json"
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

var (
	testNetwork = &models.Network{
		NetID: "not-a-network",
	}
	testExternalClient = &models.ExtClient{
		ClientID:    "testExtClient",
		Description: "ext client for testing",
	}
)

func TestNetworkExists(t *testing.T) {
	err := database.InitializeDatabase()
	if err != nil {
		t.Fatalf("error initilizing database: %s", err)
	}
	database.DeleteRecord(database.NETWORKS_TABLE_NAME, testNetwork.NetID)
	defer database.CloseDB()
	exists, err := logic.NetworkExists(testNetwork.NetID)
	if err == nil {
		t.Fatalf("expected error, received nil")
	}
	if exists {
		t.Fatalf("expected false")
	}

	err = logic.SaveNetwork(testNetwork)
	if err != nil {
		t.Fatalf("failed to save test network in databse: %s", err)
	}
	exists, err = logic.NetworkExists(testNetwork.NetID)
	if err != nil {
		t.Fatalf("expected nil, received err: %s", err)
	}
	if !exists {
		t.Fatalf("expected network to exist in database")
	}

	err = database.DeleteRecord(database.NETWORKS_TABLE_NAME, testNetwork.NetID)
	if err != nil {
		t.Fatalf("expected nil, failed to delete test network: %s", err)
	}
}

func TestGetAllExtClients(t *testing.T) {
	err := database.InitializeDatabase()
	if err != nil {
		t.Fatalf("error initilizing database: %s", err)
	}
	defer database.CloseDB()
	database.DeleteRecord(database.EXT_CLIENT_TABLE_NAME, testExternalClient.ClientID)

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
