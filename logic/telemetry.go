package logic

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/gorm"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/posthog/posthog-go"
	"golang.org/x/exp/slog"
)

var LogEvent = func(a *models.Event) {}

// posthog_pub_key - Key for sending data to PostHog
const posthog_pub_key = "phc_1vEXhPOA1P7HP5jP2dVU9xDTUqXHAelmtravyZ1vvES"

// posthog_endpoint - Endpoint of PostHog server
const posthog_endpoint = "https://app.posthog.com"

// sendTelemetry - gathers telemetry data and sends to posthog
func sendTelemetry() error {
	if Telemetry() == "off" {
		return nil
	}

	serverID := &schema.Internal{
		Key: schema.InternalKey_ServerID,
	}
	err := serverID.Get(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	// get telemetry data
	d := FetchTelemetryData()
	// get tenant admin email
	adminEmail := os.Getenv("NM_EMAIL")
	client, err := posthog.NewWithConfig(posthog_pub_key, posthog.Config{Endpoint: posthog_endpoint})
	if err != nil {
		return err
	}
	defer client.Close()

	slog.Info("sending telemetry data to posthog", "data", d)

	// send to posthog
	return client.Enqueue(posthog.Capture{
		DistinctId: serverID.Value,
		Event:      "daily checkin",
		Properties: posthog.NewProperties().
			Set("nodes", d.Nodes).
			Set("hosts", d.Hosts).
			Set("servers", d.Servers).
			Set("non-server nodes", d.Count.NonServer).
			Set("extclients", d.ExtClients).
			Set("users", d.Users).
			Set("networks", d.Networks).
			Set("linux", d.Count.Linux).
			Set("darwin", d.Count.MacOS).
			Set("windows", d.Count.Windows).
			Set("freebsd", d.Count.FreeBSD).
			Set("docker", d.Count.Docker).
			Set("k8s", d.Count.K8S).
			Set("version", d.Version).
			Set("is_ee", d.IsPro). // TODO change is_ee to is_pro for consistency, but probably needs changes in posthog
			Set("is_free_tier", false).
			Set("is_pro_trial", d.IsProTrial).
			Set("pro_trial_end_date", d.ProTrialEndDate.In(time.UTC).Format("2006-01-02")).
			Set("admin_email", adminEmail).
			Set("email", adminEmail). // needed for posthog integration with hubspot. "admin_email" can only be removed if not used in posthog
			Set("is_saas_tenant", d.IsSaasTenant).
			Set("domain", d.Domain),
	})
}

// FetchTelemetryData - fetches telemetry data: count of various object types in DB
func FetchTelemetryData() telemetryData {
	var data telemetryData

	data.IsPro = servercfg.IsPro
	data.ExtClients, _ = (&schema.ExtClientRecord{}).Count(db.WithContext(context.TODO()))
	data.Users, _ = (&schema.User{}).Count(db.WithContext(context.TODO()))
	data.Networks, _ = (&schema.Network{}).Count(db.WithContext(context.TODO()))
	data.Hosts, _ = (&schema.Host{}).Count(db.WithContext(context.TODO()))
	data.Version = servercfg.GetVersion()
	data.Servers = getServerCount()
	nodes, _ := (&schema.Node{}).ListAll(db.WithContext(context.TODO()), dbtypes.WithPreloads("Host"))
	data.Nodes = len(nodes)
	data.Count = getClientCount(nodes)
	data.ProTrialEndDate, _ = time.Parse("2006-Jan-02", "2021-Apr-01")
	data.IsSaasTenant = servercfg.DeployedByOperator()
	data.Domain = servercfg.GetNmBaseDomain()
	return data
}

// getServerCount returns number of servers from database
func getServerCount() int {
	return 1
}

func getTelemetryLastReportedAt() (time.Time, error) {
	telemetryLastReportedAt := &schema.Internal{
		Key: schema.InternalKey_TelemetryLastReportedAt,
	}
	err := telemetryLastReportedAt.Get(db.WithContext(context.TODO()))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, nil
		}

		return time.Time{}, err
	}

	telemetryLastReportedAtValue, err := time.Parse(time.RFC3339, telemetryLastReportedAt.Value)
	if err != nil {
		return time.Time{}, err
	}

	return telemetryLastReportedAtValue, nil
}

// setTelemetryLastReportedAt sets the time for the last hook run.
func setTelemetryLastReportedAt() error {
	lastHookRunAt := &schema.Internal{
		Key:   schema.InternalKey_TelemetryLastReportedAt,
		Value: time.Now().UTC().Format(time.RFC3339),
	}
	ctx := db.WithContext(context.TODO())
	if lastHookRunAt.TenantID == "" {
		lastHookRunAt.TenantID = scope.ID(DefaultScope(ctx))
	}
	return lastHookRunAt.Set(ctx)
}

// getClientCount - returns counts of nodes with various OS types and conditions
func getClientCount(nodes []schema.Node) clientCount {
	var count clientCount
	for _, node := range nodes {
		if node.Host == nil {
			continue
		}
		switch node.Host.OS {
		case "darwin":
			count.MacOS += 1
		case "windows":
			count.Windows += 1
		case "linux":
			count.Linux += 1
		case "freebsd":
			count.FreeBSD += 1
		}
	}
	return count
}

// telemetryData - What data to send to posthog
type telemetryData struct {
	Nodes           int
	Hosts           int
	ExtClients      int
	Users           int
	Count           clientCount
	Networks        int
	Servers         int
	Version         string
	IsPro           bool
	IsFreeTier      bool
	IsProTrial      bool
	ProTrialEndDate time.Time
	IsSaasTenant    bool
	Domain          string
}

// clientCount - What types of netclients we're tallying
type clientCount struct {
	MacOS     int
	Windows   int
	Linux     int
	FreeBSD   int
	K8S       int
	Docker    int
	NonServer int
}
