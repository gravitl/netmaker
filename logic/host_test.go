package logic

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/matryer/is"
)

func TestMain(m *testing.M) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	peerUpdate := make(chan *models.Node)
	go ManageZombies(context.Background())
	go func() {
		for y := range peerUpdate {
			fmt.Printf("Pointless %v\n", y)
			//do nothing
		}
	}()

	defaultOrg := schema.Organization{}
	_ = defaultOrg.CreateDefault(db.WithContext(context.TODO()))

	defaultTenant := schema.Tenant{
		OrganizationID: defaultOrg.ID,
	}
	_ = defaultTenant.CreateDefault(db.WithContext(context.TODO()))

	os.Exit(m.Run())
}

func TestCheckPorts(t *testing.T) {
	h := schema.Host{
		ID:         uuid.New(),
		EndpointIP: net.ParseIP("192.168.1.1"),
		ListenPort: 51821,
	}
	testHost := schema.Host{
		ID:         uuid.New(),
		EndpointIP: net.ParseIP("192.168.1.1"),
		ListenPort: 51830,
	}
	//not sure why this initialization is required but without it
	// RemoveHost returns database is closed
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	RemoveHost(&h, true)
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

}
