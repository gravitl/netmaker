package logic

import (
	"context"
	"encoding/base64"
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

func ensureTestMqKeys(ctx context.Context) {
	pub := &schema.Internal{Key: schema.InternalKey_MqPublicKey}
	if err := pub.Get(ctx); err != nil {
		pub.Value = base64.StdEncoding.EncodeToString([]byte("test-mq-public-key-for-logic-tests"))
		_ = pub.Set(ctx)
	}
	priv := &schema.Internal{Key: schema.InternalKey_MqPrivateKey}
	if err := priv.Get(ctx); err != nil {
		priv.Value = base64.StdEncoding.EncodeToString([]byte("test-mq-private-key-for-logic-tests"))
		_ = priv.Set(ctx)
	}
}

func TestMain(m *testing.M) {
	db.InitializeDB(schema.ListModels()...)
	defer db.CloseDB()

	ctx := db.WithContext(context.TODO())
	defaultOrg := schema.Organization{}
	if err := defaultOrg.CreateDefault(ctx); err != nil {
		// CreateDefault sets ID before the failed insert; GetDefault must run
		// on a fresh struct or gorm.First will AND in the stale ID and never match.
		defaultOrg = schema.Organization{}
		_ = defaultOrg.GetDefault(ctx)
	}

	defaultTenant := schema.Tenant{
		OrganizationID: defaultOrg.ID,
	}
	if err := defaultTenant.CreateDefault(ctx); err != nil {
		defaultTenant = schema.Tenant{}
		_ = defaultTenant.GetDefault(ctx)
	}
	ensureTestMqKeys(ctx)

	peerUpdate := make(chan *models.Node)
	go ManageZombies(context.Background())
	go func() {
		for y := range peerUpdate {
			fmt.Printf("Pointless %v\n", y)
			//do nothing
		}
	}()

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

	RemoveHost(db.WithContext(context.TODO()), &h, true)
	CreateHost(db.WithContext(context.TODO()), &h)
	t.Run("no change", func(t *testing.T) {
		is := is.New(t)
		CheckHostPorts(db.WithContext(context.TODO()), &testHost)
		t.Log(testHost.ListenPort)
		t.Log(h.ListenPort)
		is.Equal(testHost.ListenPort, 51830)
	})
	t.Run("same listen port", func(t *testing.T) {
		is := is.New(t)
		testHost.ListenPort = 51821
		CheckHostPorts(db.WithContext(context.TODO()), &testHost)
		t.Log(testHost.ListenPort)
		t.Log(h.ListenPort)
		is.Equal(testHost.ListenPort, 51822)
	})

}
