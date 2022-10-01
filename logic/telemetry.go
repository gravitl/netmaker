package logic

import (
	"encoding/json"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/posthog/posthog-go"
)

// flags to keep for telemetry
var isFreeTier bool
var isEE bool

// posthog_pub_key - Key for sending data to PostHog
const posthog_pub_key = "phc_1vEXhPOA1P7HP5jP2dVU9xDTUqXHAelmtravyZ1vvES"

// posthog_endpoint - Endpoint of PostHog server
const posthog_endpoint = "https://app.posthog.com"

// setEEForTelemetry - store EE flag without having an import cycle when used for telemetry
// (as the ee package needs the logic package as currently written).
func SetEEForTelemetry(eeFlag bool) {
	isEE = eeFlag
}

// setFreeTierForTelemetry - store free tier flag without having an import cycle when used for telemetry
// (as the ee package needs the logic package as currently written).
func SetFreeTierForTelemetry(freeTierFlag bool) {
	isFreeTier = freeTierFlag
}

// sendTelemetry - gathers telemetry data and sends to posthog
func sendTelemetry() error {
	if servercfg.Telemetry() == "off" {
		return nil
	}

	var telRecord, err = fetchTelemetryRecord()
	if err != nil {
		return err
	}
	// get telemetry data
	d, err := fetchTelemetryData()
	if err != nil {
		return err
	}
	client, err := posthog.NewWithConfig(posthog_pub_key, posthog.Config{Endpoint: posthog_endpoint})
	if err != nil {
		return err
	}
	defer client.Close()

	// send to posthog
	return client.Enqueue(posthog.Capture{
		DistinctId: telRecord.UUID,
		Event:      "daily checkin",
		Properties: posthog.NewProperties().
			Set("nodes", d.Nodes).
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
			Set("is_ee", isEE).
			Set("is_free_tier", isFreeTier),
	})
}

// fetchTelemetry - fetches telemetry data: count of various object types in DB
func fetchTelemetryData() (telemetryData, error) {
	var data telemetryData

	data.ExtClients = getDBLength(database.EXT_CLIENT_TABLE_NAME)
	data.Users = getDBLength(database.USERS_TABLE_NAME)
	data.Networks = getDBLength(database.NETWORKS_TABLE_NAME)
	data.Version = servercfg.GetVersion()
	data.Servers = GetServerCount()
	nodes, err := GetAllNodes()
	if err == nil {
		data.Nodes = len(nodes)
		data.Count = getClientCount(nodes)
	}
	return data, err
}

// setTelemetryTimestamp - Give the entry in the DB a new timestamp
func setTelemetryTimestamp(telRecord *models.Telemetry) error {
	lastsend := time.Now().Unix()
	var serverTelData = models.Telemetry{
		UUID:           telRecord.UUID,
		LastSend:       lastsend,
		TrafficKeyPriv: telRecord.TrafficKeyPriv,
		TrafficKeyPub:  telRecord.TrafficKeyPub,
	}
	jsonObj, err := json.Marshal(&serverTelData)
	if err != nil {
		return err
	}
	err = database.Insert(database.SERVER_UUID_RECORD_KEY, string(jsonObj), database.SERVER_UUID_TABLE_NAME)
	return err
}

// getClientCount - returns counts of nodes with various OS types and conditions
func getClientCount(nodes []models.Node) clientCount {
	var count clientCount
	for _, node := range nodes {
		switch node.OS {
		case "darwin":
			count.MacOS += 1
		case "windows":
			count.Windows += 1
		case "linux":
			count.Linux += 1
		case "freebsd":
			count.FreeBSD += 1
		}
		if !(node.IsServer == "yes") {
			count.NonServer += 1
		}
	}
	return count
}

// fetchTelemetryRecord - get the existing UUID and Timestamp from the DB
func fetchTelemetryRecord() (models.Telemetry, error) {
	var rawData string
	var telObj models.Telemetry
	var err error
	rawData, err = database.FetchRecord(database.SERVER_UUID_TABLE_NAME, database.SERVER_UUID_RECORD_KEY)
	if err != nil {
		return telObj, err
	}
	err = json.Unmarshal([]byte(rawData), &telObj)
	return telObj, err
}

// getDBLength - get length of DB to get count of objects
func getDBLength(dbname string) int {
	data, err := database.FetchRecords(dbname)
	if err != nil {
		return 0
	}
	return len(data)
}

// telemetryData - What data to send to posthog
type telemetryData struct {
	Nodes      int
	ExtClients int
	Users      int
	Count      clientCount
	Networks   int
	Servers    int
	Version    string
	IsEE       bool
	IsFreeTier bool
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
