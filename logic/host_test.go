package logic

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/matryer/is"
)

func TestMain(m *testing.M) {
	database.InitializeDatabase()
	defer database.CloseDB()
	peerUpdate := make(chan *models.Node)
	go ManageZombies(context.Background(), peerUpdate)
	go func() {
		for y := range peerUpdate {
			fmt.Printf("Pointless %v\n", y)
			//do nothing
		}
	}()

	os.Exit(m.Run())
}

func TestCheckPorts(t *testing.T) {
	h := models.Host{
		ID:         uuid.New(),
		EndpointIP: net.ParseIP("192.168.1.1"),
		ListenPort: 51821,
	}
	testHost := models.Host{
		ID:         uuid.New(),
		EndpointIP: net.ParseIP("192.168.1.1"),
		ListenPort: 51830,
	}
	//not sure why this initialization is required but without it
	// RemoveHost returns database is closed
	database.InitializeDatabase()
	RemoveHost(&h)
	CreateHost(&h)
	t.Run("no change", func(t *testing.T) {
		is := is.New(t)
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort)
		t.Log(h.ListenPort)
		is.Equal(testHost.ListenPort, 51830)
	})
	t.Run("same listen port", func(t *testing.T) {
		is := is.New(t)
		testHost.ListenPort = 51821
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort)
		t.Log(h.ListenPort)
		is.Equal(testHost.ListenPort, 51822)
	})
	t.Run("same proxy port", func(t *testing.T) {
		is := is.New(t)
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort)
		t.Log(h.ListenPort)
		is.Equal(testHost.ListenPort, 51822)
	})
	t.Run("listenport equals proxy port", func(t *testing.T) {
		is := is.New(t)
		testHost.ListenPort = maxPort
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort)
		t.Log(h.ListenPort)
		is.Equal(testHost.ListenPort, minPort)
	})
	t.Run("proxyport equals listenport", func(t *testing.T) {
		is := is.New(t)
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort)
		t.Log(h.ListenPort)
		is.Equal(testHost.ListenPort, minPort)
	})
}
