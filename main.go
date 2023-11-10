// -build ee
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/gravitl/netmaker/auth"
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
	"golang.org/x/exp/slog"
)

var version = "v0.21.2"

// Start DB Connection and start API Request Handler
func main() {
	absoluteConfigPath := flag.String("c", "", "absolute path to configuration file")
	flag.Parse()
	setupConfig(*absoluteConfigPath)
	servercfg.SetVersion(version)
	fmt.Println(models.RetrieveLogo()) // print the logo
	initialize()                       // initial db and acls
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
	migrate.Run()

	logic.SetJWTSecret()

	var authProvider = auth.InitializeAuthProvider()
	if authProvider != "" {
		logger.Log(0, "OAuth provider,", authProvider+",", "initialized")
	} else {
		logger.Log(0, "no OAuth provider found or not configured, continuing without OAuth")
	}

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
	wg.Add(1)
	go runMessageQueue(ctx, wg)
	peerUpdate := make(chan *models.Node)
	wg.Add(1)
	go logic.ManageZombies(ctx, wg, peerUpdate)
	wg.Add(1)
	go logic.DeleteExpiredNodes(ctx, wg, peerUpdate)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case nodeUpdate := <-peerUpdate:
				if err := mq.NodeUpdate(nodeUpdate); err != nil {
					logger.Log(0, "failed to send peer update for deleted node: ", nodeUpdate.ID.String(), err.Error())
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	if !servercfg.IsRestBackend() && !servercfg.IsMessageQueueBackend() {
		logger.Log(0, "No Server Mode selected, so nothing is being served! Set Rest mode (REST_BACKEND) or MessageQueue (MESSAGEQUEUE_BACKEND) to 'true'.")
	}

	wg.Add(1)
	go logic.StartHookManager(ctx, wg)
}

// Should we be using a context vice a waitgroup????????????
func runMessageQueue(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	go mq.Keepalive(ctx)
	defer mq.CloseClient()
	for {
		brokerHost, _ := servercfg.GetMessageQueueEndpoint()
		logger.Log(0, "connecting to mq broker at", brokerHost)
		mq.SetupMQTT()
		if mq.IsConnected() {
			logger.Log(0, "connected to MQ Broker")
		} else {
			logger.FatalLog("error connecting to MQ Broker")
		}
		select {
		case <-mq.ResetCh:
			slog.Info("## Resetting MQ Connection")
			mq.CloseClient()
			time.Sleep(time.Second * 2)
			continue
		case <-ctx.Done():
			return
		}
	}

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
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{AddSource: true, ReplaceAttr: replace, Level: logLevel}))
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
