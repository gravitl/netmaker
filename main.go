// -build ee
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"

	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/config"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/queue"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
	stunserver "github.com/gravitl/netmaker/stun-server"
)

var version = "v0.18.0"

// Start DB Connection and start API Request Handler
func main() {
	absoluteConfigPath := flag.String("c", "", "absolute path to configuration file")
	flag.Parse()
	setupConfig(*absoluteConfigPath)
	servercfg.SetVersion(version)
	fmt.Println(models.RetrieveLogo()) // print the logo
	// fmt.Println(models.ProLogo())
	initialize() // initial db and acls; gen cert if required
	setGarbageCollection()
	setVerbosity()
	defer database.CloseDB()
	startControllers() // start the api endpoint and mq
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

	logic.SetJWTSecret()

	if err = pro.InitializeGroups(); err != nil {
		logger.Log(0, "could not initialize default user group, \"*\"")
	}

	err = logic.TimerCheckpoint()
	if err != nil {
		logger.Log(1, "Timer error occurred: ", err.Error())
	}

	logic.EnterpriseCheck()

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

func startControllers() {
	var waitnetwork sync.WaitGroup
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
		waitnetwork.Add(1)
		go controller.HandleRESTRequests(&waitnetwork)
	}
	// Run External or Internal MessageQueue
	waitnetwork.Add(1)
	go runMessageQueue(&waitnetwork)

	if !servercfg.IsAgentBackend() && !servercfg.IsRestBackend() && !servercfg.IsMessageQueueBackend() {
		logger.Log(0, "No Server Mode selected, so nothing is being served! Set Agent mode (AGENT_BACKEND) or Rest mode (REST_BACKEND) or MessageQueue (MESSAGEQUEUE_BACKEND) to 'true'.")
	}

	// starts the stun server
	waitnetwork.Add(1)
	go stunserver.Start(&waitnetwork)
	if servercfg.IsProxyEnabled() {

		waitnetwork.Add(1)
		go func() {
			defer waitnetwork.Done()
			_, cancel := context.WithCancel(context.Background())
			waitnetwork.Add(1)

			//go nmproxy.Start(ctx, logic.ProxyMgmChan, servercfg.GetAPIHost())
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
			<-quit
			cancel()
		}()
	}

	waitnetwork.Wait()
}

// Should we be using a context vice a waitgroup????????????
func runMessageQueue(wg *sync.WaitGroup) {
	defer wg.Done()
	ctx, cancel := context.WithCancel(context.Background())
	if servercfg.IsMessageQueueBackend() { // connect to external broker
		brokerHost, secure := servercfg.GetMessageQueueEndpoint()
		logger.Log(0, "connecting to mq broker at", brokerHost, "with TLS?", fmt.Sprintf("%v", secure))
		mq.SetupMQTT()
		go mq.Keepalive(ctx)
	} else { // use internal queue system
		queue.StartQueue(ctx)
	}
	go func() {
		peerUpdate := make(chan *models.Node)
		go logic.ManageZombies(ctx, peerUpdate)
		if servercfg.IsMessageQueueBackend() {
			for nodeUpdate := range peerUpdate {
				if err := mq.NodeUpdate(nodeUpdate); err != nil {
					logger.Log(0, "failed to send peer update for deleted node: ", nodeUpdate.ID.String(), err.Error())
				}
			}
		}
	}()
	go logic.PurgePendingNodes(ctx)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
	<-quit
	cancel()
	logger.Log(0, "Message Queue shutting down")
}

func setVerbosity() {
	verbose := int(servercfg.GetVerbosity())
	logger.Verbosity = verbose
}

func setGarbageCollection() {
	_, gcset := os.LookupEnv("GOGC")
	if !gcset {
		debug.SetGCPercent(ncutils.DEFAULT_GC_PERCENT)
	}
}
