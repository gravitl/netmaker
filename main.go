package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
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
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
	"github.com/gravitl/netmaker/tls"
)

var version = "dev"

// Start DB Connection and start API Request Handler
func main() {
	absoluteConfigPath := flag.String("c", "", "absolute path to configuration file")
	flag.Parse()

	setupConfig(*absoluteConfigPath)
	servercfg.SetVersion(version)
	fmt.Println(models.RetrieveLogo()) // print the logo
	initialize()                       // initial db and acls; gen cert if required
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
		logger.FatalLog("Error connecting to database")
	}
	logger.Log(0, "database successfully connected")
	logic.SetJWTSecret()

	err = logic.TimerCheckpoint()
	if err != nil {
		logger.Log(1, "Timer error occurred: ", err.Error())
	}
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

	if servercfg.IsClientMode() != "off" {
		output, err := ncutils.RunCmd("id -u", true)
		if err != nil {
			logger.FatalLog("Error running 'id -u' for prereq check. Please investigate or disable client mode.", output, err.Error())
		}
		uid, err := strconv.Atoi(string(output[:len(output)-1]))
		if err != nil {
			logger.FatalLog("Error retrieving uid from 'id -u' for prereq check. Please investigate or disable client mode.", err.Error())
		}
		if uid != 0 {
			logger.FatalLog("To run in client mode requires root privileges. Either disable client mode or run with sudo.")
		}
		if err := serverctl.InitServerNetclient(); err != nil {
			logger.FatalLog("Did not find netclient to use CLIENT_MODE")
		}
	}
	// initialize iptables to ensure gateways work correctly and mq is forwarded if containerized
	if servercfg.ManageIPTables() != "off" {
		if err = serverctl.InitIPTables(true); err != nil {
			logger.FatalLog("Unable to initialize iptables on host:", err.Error())
		}
	}

	if servercfg.IsDNSMode() {
		err := functions.SetDNSDir()
		if err != nil {
			logger.FatalLog(err.Error())
		}
	}

	if err = genCerts(); err != nil {
		logger.Log(0, "something went wrong when generating broker certs", err.Error())
	}

	if servercfg.IsMessageQueueBackend() {
		if err = mq.ServerStartNotify(); err != nil {
			logger.Log(0, "error occurred when notifying nodes of startup", err.Error())
		}
	}
	logic.InitalizeZombies()
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

	//Run MessageQueue
	if servercfg.IsMessageQueueBackend() {
		waitnetwork.Add(1)
		go runMessageQueue(&waitnetwork)
	}

	if !servercfg.IsAgentBackend() && !servercfg.IsRestBackend() && !servercfg.IsMessageQueueBackend() {
		logger.Log(0, "No Server Mode selected, so nothing is being served! Set Agent mode (AGENT_BACKEND) or Rest mode (REST_BACKEND) or MessageQueue (MESSAGEQUEUE_BACKEND) to 'true'.")
	}

	waitnetwork.Wait()
}

// Should we be using a context vice a waitgroup????????????
func runMessageQueue(wg *sync.WaitGroup) {
	defer wg.Done()
	brokerHost, secure := servercfg.GetMessageQueueEndpoint()
	logger.Log(0, "connecting to mq broker at", brokerHost, "with TLS?", fmt.Sprintf("%v", secure))
	mq.SetupMQTT()
	ctx, cancel := context.WithCancel(context.Background())
	go mq.Keepalive(ctx)
	go logic.ManageZombies(ctx)
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

func genCerts() error {
	logger.Log(0, "checking keys and certificates")
	var private *ed25519.PrivateKey
	var err error

	// == ROOT key handling ==

	private, err = serverctl.ReadKeyFromDB(tls.ROOT_KEY_NAME)
	if errors.Is(err, os.ErrNotExist) || database.IsEmptyRecord(err) {
		logger.Log(0, "generating new root key")
		_, newKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		private = &newKey
	} else if err != nil {
		return err
	}
	logger.Log(2, "saving root.key")
	if err := serverctl.SaveKey(functions.GetNetmakerPath()+ncutils.GetSeparator(), tls.ROOT_KEY_NAME, *private); err != nil {
		return err
	}

	// == ROOT cert handling ==

	ca, err := serverctl.ReadCertFromDB(tls.ROOT_PEM_NAME)
	//if cert doesn't exist or will expire within 10 days --- but can't do this as clients won't be able to connect
	//if errors.Is(err, os.ErrNotExist) || cert.NotAfter.Before(time.Now().Add(time.Hour*24*10)) {
	if errors.Is(err, os.ErrNotExist) || database.IsEmptyRecord(err) || ca.NotAfter.Before(time.Now().Add(time.Hour*24*10)) {
		logger.Log(0, "generating new root CA")
		caName := tls.NewName("CA Root", "US", "Gravitl")
		csr, err := tls.NewCSR(*private, caName)
		if err != nil {
			return err
		}
		rootCA, err := tls.SelfSignedCA(*private, csr, tls.CERTIFICATE_VALIDITY)
		if err != nil {
			return err
		}
		ca = rootCA
	} else if err != nil {
		return err
	}
	logger.Log(2, "saving root.pem")
	if err := serverctl.SaveCert(functions.GetNetmakerPath()+ncutils.GetSeparator(), tls.ROOT_PEM_NAME, ca); err != nil {
		return err
	}

	// == SERVER cert handling ==

	cert, err := serverctl.ReadCertFromDB(tls.SERVER_PEM_NAME)
	if errors.Is(err, os.ErrNotExist) || database.IsEmptyRecord(err) || cert.NotAfter.Before(time.Now().Add(time.Hour*24*10)) {
		//gen new key
		logger.Log(0, "generating new server key/certificate")
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		serverName := tls.NewCName(servercfg.GetServer())
		csr, err := tls.NewCSR(key, serverName)
		if err != nil {
			return err
		}
		newCert, err := tls.NewEndEntityCert(*private, csr, ca, tls.CERTIFICATE_VALIDITY)
		if err != nil {
			return err
		}
		if err := serverctl.SaveKey(functions.GetNetmakerPath()+ncutils.GetSeparator(), tls.SERVER_KEY_NAME, key); err != nil {
			return err
		}
		cert = newCert
	} else if err != nil {
		return err
	} else if err == nil {
		if serverKey, err := serverctl.ReadKeyFromDB(tls.SERVER_KEY_NAME); err == nil {
			logger.Log(2, "saving server.key")
			if err := serverctl.SaveKey(functions.GetNetmakerPath()+ncutils.GetSeparator(), tls.SERVER_KEY_NAME, *serverKey); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	logger.Log(2, "saving server.pem")
	if err := serverctl.SaveCert(functions.GetNetmakerPath()+ncutils.GetSeparator(), tls.SERVER_PEM_NAME, cert); err != nil {
		return err
	}

	// == SERVER-CLIENT connection cert handling ==

	serverClientCert, err := serverctl.ReadCertFromDB(tls.SERVER_CLIENT_PEM)
	if errors.Is(err, os.ErrNotExist) || database.IsEmptyRecord(err) || serverClientCert.NotAfter.Before(time.Now().Add(time.Hour*24*10)) {
		//gen new key
		logger.Log(0, "generating new server client key/certificate")
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		serverName := tls.NewCName(tls.SERVER_CLIENT_ENTRY)
		csr, err := tls.NewCSR(key, serverName)
		if err != nil {
			return err
		}
		newServerClientCert, err := tls.NewEndEntityCert(*private, csr, ca, tls.CERTIFICATE_VALIDITY)
		if err != nil {
			return err
		}

		if err := serverctl.SaveKey(functions.GetNetmakerPath()+ncutils.GetSeparator(), tls.SERVER_CLIENT_KEY, key); err != nil {
			return err
		}
		serverClientCert = newServerClientCert
	} else if err != nil {
		return err
	} else if err == nil {
		logger.Log(2, "saving serverclient.key")
		if serverClientKey, err := serverctl.ReadKeyFromDB(tls.SERVER_CLIENT_KEY); err == nil {
			if err := serverctl.SaveKey(functions.GetNetmakerPath()+ncutils.GetSeparator(), tls.SERVER_CLIENT_KEY, *serverClientKey); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	logger.Log(2, "saving serverclient.pem")
	if err := serverctl.SaveCert(functions.GetNetmakerPath()+ncutils.GetSeparator(), tls.SERVER_CLIENT_PEM, serverClientCert); err != nil {
		return err
	}

	logger.Log(1, "ensure the root.pem, root.key, server.pem, and server.key files are updated on your broker")

	return serverctl.SetClientTLSConf(
		functions.GetNetmakerPath()+ncutils.GetSeparator()+tls.SERVER_CLIENT_PEM,
		functions.GetNetmakerPath()+ncutils.GetSeparator()+tls.SERVER_CLIENT_KEY,
		ca,
	)
}
