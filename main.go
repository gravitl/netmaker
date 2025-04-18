// -build ee
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/cilium/fake"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/config"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/migrate"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
	_ "go.uber.org/automaxprocs"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/exp/slog"
)

var version = "v0.90.0"

//	@title			NetMaker
//	@version		0.90.0
//	@description	NetMaker API Docs
//	@tag.name	    APIUsage
//	@tag.description.markdown
//	@tag.name	    Authentication
//	@tag.description.markdown
//	@tag.name	    Pricing
//	@tag.description.markdown
//  @host      api.demo.netmaker.io

// Start DB Connection and start API Request Handler
func main() {
	absoluteConfigPath := flag.String("c", "", "absolute path to configuration file")
	flag.Parse()
	setupConfig(*absoluteConfigPath)
	servercfg.SetVersion(version)
	fmt.Println(models.RetrieveLogo()) // print the logo
	initialize()                       // initial db and acls
	logic.SetAllocatedIpMap()
	defer logic.ClearAllocatedIpMap()
	setGarbageCollection()
	setVerbosity()
	if servercfg.DeployedByOperator() && !servercfg.IsPro {
		logic.SetFreeTierLimits()
	}
	defer database.CloseDB()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	var waitGroup sync.WaitGroup
	startControllers(&waitGroup, ctx) // start the api endpoint and mq and stun
	startHooks()
	<-ctx.Done()
	waitGroup.Wait()
}

func setupConfig(absoluteConfigPath string) {
	if len(absoluteConfigPath) > 0 {
		cfg, err := config.ReadConfig(absoluteConfigPath)
		if err != nil {
			logger.Log(0, fmt.Sprintf("failed parsing config at: %s", absoluteConfigPath))
			return
		}
		config.Config = cfg
	}
}

func startHooks() {
	err := logic.TimerCheckpoint()
	if err != nil {
		logger.Log(1, "Timer error occurred: ", err.Error())
	}
	logic.EnterpriseCheck()
}

func initialize() { // Client Mode Prereq Check
	var err error

	if servercfg.GetMasterKey() == "" {
		logger.Log(0, "warning: MASTER_KEY not set, this could make account recovery difficult")
	}

	if servercfg.GetNodeID() == "" {
		logger.FatalLog("error: must set NODE_ID, currently blank")
	}

	if err = database.InitializeDatabase(); err != nil {
		logger.FatalLog("Error connecting to database: ", err.Error())
	}
	logger.Log(0, "database successfully connected")

	//initialize cache
	_, _ = logic.GetNetworks()
	_, _ = logic.GetAllNodes()
	_, _ = logic.GetAllHosts()
	_, _ = logic.GetAllExtClients()
	_ = logic.ListAcls()
	_, _ = logic.GetAllEnrollmentKeys()

	migrate.Run()

	logic.SetJWTSecret()
	logic.InitialiseRoles()
	logic.IntialiseGroups()
	err = serverctl.SetDefaults()
	if err != nil {
		logger.FatalLog("error setting defaults: ", err.Error())
	}

	if servercfg.IsDNSMode() {
		err := functions.SetDNSDir()
		if err != nil {
			logger.FatalLog(err.Error())
		}
	}

	if servercfg.IsMessageQueueBackend() {
		if err = mq.ServerStartNotify(); err != nil {
			logger.Log(0, "error occurred when notifying nodes of startup", err.Error())
		}
	}
	go GenerateFakeData()
}
func GenerateFakeData() {
	if os.Getenv("GENERATE_FAKE") != "true" {
		return
	}
	var network models.Network
	network.NetID = "mock"
	network.AddressRange = "10.0.0.0/8"

	network.IsIPv4 = "yes"
	logic.CreateNetwork(network)
	//add new network to allocated ip map
	go logic.AddNetworkToAllocatedIpMap(network.NetID)
	pub, _, err := box.GenerateKey(rand.Reader)
	if err != nil {
		logger.FatalLog("error generating traffic keys", err.Error())
	}
	bytes, err := ncutils.ConvertKeyToBytes(pub)
	if err != nil {
		logger.FatalLog("error generating traffic keys", err.Error())
	}
	hosts, _ := logic.GetAllHosts()
	for i := len(hosts); i <= 600; i++ {
		h := models.Host{
			ID:               uuid.New(),
			Name:             fake.App(),
			ListenPort:       51821,
			HostPass:         fake.AlphaNum(8),
			Version:          version,
			OS:               models.OS_Types.Linux,
			EndpointIP:       net.ParseIP(fake.IP(fake.WithIPv4())),
			EndpointIPv6:     net.ParseIP(fake.IP(fake.WithIPv6())),
			TrafficKeyPublic: bytes,
		}
		logic.CreateHost(&h)
		_, _ = logic.UpdateHostNetwork(&h, network.NetID, true)
		time.Sleep(time.Millisecond * 600)

	}
	var network2 models.Network
	network2.NetID = "devops"
	network2.AddressRange = "100.24.0.0/16"
	network2.IsIPv4 = "yes"
	logic.CreateNetwork(network2)
	//add new network to allocated ip map
	go logic.AddNetworkToAllocatedIpMap(network.NetID)
	nodes, _ := logic.GetNetworkNodes(network2.NetID)
	count := len(nodes)
	hosts, _ = logic.GetAllHosts()
	for _, hI := range hosts {
		_, _ = logic.UpdateHostNetwork(&hI, network2.NetID, true)
		if count >= 200 {
			break
		}
		count++
		time.Sleep(time.Millisecond * 600)
	}

}

func startControllers(wg *sync.WaitGroup, ctx context.Context) {
	if servercfg.IsDNSMode() {
		err := logic.SetDNS()
		if err != nil {
			logger.Log(0, "error occurred initializing DNS: ", err.Error())
		}
	}

	//Run Rest Server
	if servercfg.IsRestBackend() {
		if !servercfg.DisableRemoteIPCheck() && servercfg.GetAPIHost() == "127.0.0.1" {
			err := servercfg.SetHost()
			if err != nil {
				logger.FatalLog("Unable to Set host. Exiting...", err.Error())
			}
		}
		wg.Add(1)
		go controller.HandleRESTRequests(wg, ctx)
	}
	//Run MessageQueue
	if servercfg.IsMessageQueueBackend() {
		wg.Add(1)
		go runMessageQueue(wg, ctx)
	}

	if !servercfg.IsRestBackend() && !servercfg.IsMessageQueueBackend() {
		logger.Log(
			0,
			"No Server Mode selected, so nothing is being served! Set Rest mode (REST_BACKEND) or MessageQueue (MESSAGEQUEUE_BACKEND) to 'true'.",
		)
	}

	wg.Add(1)
	go logic.StartHookManager(ctx, wg)
}

// Should we be using a context vice a waitgroup????????????
func runMessageQueue(wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()
	brokerHost, _ := servercfg.GetMessageQueueEndpoint()
	logger.Log(0, "connecting to mq broker at", brokerHost)
	mq.SetupMQTT(true)
	if mq.IsConnected() {
		logger.Log(0, "connected to MQ Broker")
	} else {
		logger.FatalLog("error connecting to MQ Broker")
	}
	defer mq.CloseClient()
	go mq.Keepalive(ctx)
	go func() {
		peerUpdate := make(chan *models.Node)
		go logic.ManageZombies(ctx, peerUpdate)
		go logic.DeleteExpiredNodes(ctx, peerUpdate)
		for nodeUpdate := range peerUpdate {
			if nodeUpdate == nil {
				continue
			}
			node := nodeUpdate
			node.Action = models.NODE_DELETE
			node.PendingDelete = true
			if err := mq.NodeUpdate(node); err != nil {
				logger.Log(
					0,
					"failed to send peer update for deleted node: ",
					node.ID.String(),
					err.Error(),
				)
			}
			if err := logic.DeleteNode(node, true); err != nil {
				slog.Error(
					"error deleting expired node",
					"nodeid",
					node.ID.String(),
					"error",
					err.Error(),
				)
			}
			go mq.PublishDeletedNodePeerUpdate(node)
		}
	}()
	<-ctx.Done()
	logger.Log(0, "Message Queue shutting down")
}

func setVerbosity() {
	verbose := int(servercfg.GetVerbosity())
	logger.Verbosity = verbose
	logLevel := &slog.LevelVar{}
	replace := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.SourceKey {
			a.Value = slog.StringValue(filepath.Base(a.Value.String()))
		}
		return a
	}
	logger := slog.New(
		slog.NewJSONHandler(
			os.Stderr,
			&slog.HandlerOptions{AddSource: true, ReplaceAttr: replace, Level: logLevel},
		),
	)
	slog.SetDefault(logger)
	switch verbose {
	case 4:
		logLevel.Set(slog.LevelDebug)
	case 3:
		logLevel.Set(slog.LevelInfo)
	case 2:
		logLevel.Set(slog.LevelWarn)
	default:
		logLevel.Set(slog.LevelError)
	}
}

func setGarbageCollection() {
	_, gcset := os.LookupEnv("GOGC")
	if !gcset {
		debug.SetGCPercent(ncutils.DEFAULT_GC_PERCENT)
	}
}
