package serverctl

import (
	"encoding/json"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/posthog/posthog-go"
)

const POSTHOG_PUB_KEY = "phc_1vEXhPOA1P7HP5jP2dVU9xDTUqXHAelmtravyZ1vvES"
const POSTHOG_ENDPOINT = "https://app.posthog.com"
const TELEMETRY_HOURS_BETWEEN_SEND = 24

// TelemetryCheckpoint - Checks if 24 hours has passed since telemetry was last sent. If so, sends telemetry data to posthog
func TelemetryCheckpoint() error {

	// if telemetry is turned off, return without doing anything
	if servercfg.Telemetry() == "off" {
		return nil
	}
	// get the telemetry record in the DB, which contains a timestamp
	telRecord, err := fetchTelemetryRecord()
	if err != nil {
		return err
	}
	sendtime := time.Unix(telRecord.LastSend, 0).Add(time.Hour * time.Duration(TELEMETRY_HOURS_BETWEEN_SEND))
	// can set to 2 minutes for testing
	//sendtime := time.Unix(telRecord.LastSend, 0).Add(time.Minute * 2)
	enoughTimeElapsed := time.Now().After(sendtime)
	// if more than 24 hours has elapsed, send telemetry to posthog
	if enoughTimeElapsed {
		err = sendTelemetry(telRecord.UUID)
		if err != nil {
			logger.Log(1, err.Error())
		}
	}
	return nil
}

// sendTelemetry - gathers telemetry data and sends to posthog
func sendTelemetry(serverUUID string) error {
	// get telemetry data
	d, err := fetchTelemetryData()
	if err != nil {
		return err
	}
	client, err := posthog.NewWithConfig(POSTHOG_PUB_KEY, posthog.Config{Endpoint: POSTHOG_ENDPOINT})
	if err != nil {
		return err
	}
	defer client.Close()

	// send to posthog
	err = client.Enqueue(posthog.Capture{
		DistinctId: serverUUID,
		Event:      "daily checkin",
		Properties: posthog.NewProperties().
			Set("nodes", d.Nodes).
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
			Set("version", d.Version),
	})
	if err != nil {
		return err
	}
	//set telemetry timestamp for server, restarts 24 hour cycle
	return setTelemetryTimestamp(serverUUID)
}

// fetchTelemetry - fetches telemetry data: count of various object types in DB
func fetchTelemetryData() (telemetryData, error) {
	var data telemetryData

	data.ExtClients = getDBLength(database.EXT_CLIENT_TABLE_NAME)
	data.Users = getDBLength(database.USERS_TABLE_NAME)
	data.Networks = getDBLength(database.NETWORKS_TABLE_NAME)
	data.Version = servercfg.GetVersion()
	nodes, err := logic.GetAllNodes()
	if err == nil {
		data.Nodes = len(nodes)
		data.Count = getClientCount(nodes)
	}
	return data, err
}

// setTelemetryTimestamp - Give the entry in the DB a new timestamp
func setTelemetryTimestamp(uuid string) error {
	lastsend := time.Now().Unix()
	var serverTelData = models.Telemetry{
		UUID:     uuid,
		LastSend: lastsend,
	}
	jsonObj, err := json.Marshal(serverTelData)
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
		case "macos":
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

// TelemetryData - What data to send to posthog
type telemetryData struct {
	Nodes      int
	ExtClients int
	Users      int
	Count      clientCount
	Networks   int
	Version    string
}

// ClientCount - What types of netclients we're tallying
type clientCount struct {
	MacOS     int
	Windows   int
	Linux     int
	FreeBSD   int
	K8S       int
	Docker    int
	NonServer int
}
