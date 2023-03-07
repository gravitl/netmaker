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
		ID:              uuid.New(),
		EndpointIP:      net.ParseIP("192.168.1.1"),
		ListenPort:      51821,
		ProxyListenPort: maxPort,
	}
	testHost := models.Host{
		ID:              uuid.New(),
		EndpointIP:      net.ParseIP("192.168.1.1"),
		ListenPort:      51830,
		ProxyListenPort: 51730,
	}
	CreateHost(&h)
	t.Run("no change", func(t *testing.T) {
		is := is.New(t)
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort, testHost.ProxyListenPort)
		t.Log(h.ListenPort, h.ProxyListenPort)
		is.Equal(testHost.ListenPort, 51830)
		is.Equal(testHost.ProxyListenPort, 51730)
	})
	t.Run("same listen port", func(t *testing.T) {
		is := is.New(t)
		testHost.ListenPort = 51821
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort, testHost.ProxyListenPort)
		t.Log(h.ListenPort, h.ProxyListenPort)
		is.Equal(testHost.ListenPort, 51822)
		is.Equal(testHost.ProxyListenPort, 51730)
	})
	t.Run("same proxy port", func(t *testing.T) {
		is := is.New(t)
		testHost.ProxyListenPort = 65535
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort, testHost.ProxyListenPort)
		t.Log(h.ListenPort, h.ProxyListenPort)
		is.Equal(testHost.ListenPort, 51822)
		is.Equal(testHost.ProxyListenPort, minPort)
	})
	t.Run("listenport equals proxy port", func(t *testing.T) {
		is := is.New(t)
		testHost.ListenPort = maxPort
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort, testHost.ProxyListenPort)
		t.Log(h.ListenPort, h.ProxyListenPort)
		is.Equal(testHost.ListenPort, minPort)
		is.Equal(testHost.ProxyListenPort, minPort+1)
	})
	t.Run("proxyport equals listenport", func(t *testing.T) {
		is := is.New(t)
		testHost.ProxyListenPort = 51821
		CheckHostPorts(&testHost)
		t.Log(testHost.ListenPort, testHost.ProxyListenPort)
		t.Log(h.ListenPort, h.ProxyListenPort)
		is.Equal(testHost.ListenPort, minPort)
		is.Equal(testHost.ProxyListenPort, 51822)
	})
}
