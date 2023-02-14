package logic

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/matryer/is"
)

func TestCheckPorts(t *testing.T) {
	initialize()
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
		is.Equal(testHost.ListenPort, 51830)
		is.Equal(testHost.ProxyListenPort, 51730)
	})
	t.Run("same listen port", func(t *testing.T) {
		is := is.New(t)
		testHost.ListenPort = 51821
		CheckHostPorts(&testHost)
		is.Equal(testHost.ListenPort, 51822)
		is.Equal(testHost.ProxyListenPort, 51730)
	})
	t.Run("same proxy port", func(t *testing.T) {
		is := is.New(t)
		testHost.ProxyListenPort = 65535
		CheckHostPorts(&testHost)
		is.Equal(testHost.ListenPort, 51822)
		is.Equal(testHost.ProxyListenPort, minPort)
	})
	t.Run("listenport equals proxy port", func(t *testing.T) {
		is := is.New(t)
		testHost.ListenPort = maxPort
		CheckHostPorts(&testHost)
		is.Equal(testHost.ListenPort, minPort)
		is.Equal(testHost.ProxyListenPort, minPort+1)
	})
	t.Run("proxyport equals listenport", func(t *testing.T) {
		is := is.New(t)
		testHost.ProxyListenPort = 51821
		CheckHostPorts(&testHost)
		is.Equal(testHost.ListenPort, minPort)
		is.Equal(testHost.ProxyListenPort, 51822)
	})
}
