package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/gravitl/netmaker/auth"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
	"google.golang.org/grpc"
)

// Start DB Connection and start API Request Handler
func main() {
	fmt.Println(models.RetrieveLogo()) // print the logo
	initialize()                       // initial db and grpc server
	setGarbageCollection()
	defer database.CloseDB()
	startControllers() // start the grpc or rest endpoints
}

func initialize() { // Client Mode Prereq Check
	var err error

	if err = database.InitializeDatabase(); err != nil {
		logger.FatalLog("Error connecting to database")
	}
	logger.Log(0, "database successfully connected")

	var authProvider = auth.InitializeAuthProvider()
	if authProvider != "" {
		logger.Log(0, "OAuth provider, ", authProvider, ", initialized")
	} else {
		logger.Log(0, "no OAuth provider found or not configured, continuing without OAuth")
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

	if servercfg.IsDNSMode() {
		err := functions.SetDNSDir()
		if err != nil {
			logger.FatalLog(err.Error())
		}
	}
}

func startControllers() {
	var waitnetwork sync.WaitGroup
	//Run Agent Server
	if servercfg.IsAgentBackend() {
		if !(servercfg.DisableRemoteIPCheck()) && servercfg.GetGRPCHost() == "127.0.0.1" {
			err := servercfg.SetHost()
			if err != nil {
				logger.FatalLog("Unable to Set host. Exiting...", err.Error())
			}
		}
		waitnetwork.Add(1)
		go runGRPC(&waitnetwork)
	}

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

	if !servercfg.IsAgentBackend() && !servercfg.IsRestBackend() {
		logger.Log(0, "No Server Mode selected, so nothing is being served! Set either Agent mode (AGENT_BACKEND) or Rest mode (REST_BACKEND) to 'true'.")
	}

	if servercfg.IsClientMode() == "on" {
		var checkintime = time.Duration(servercfg.GetServerCheckinInterval()) * time.Second
		for { // best effort currently
			var serverGroup sync.WaitGroup
			serverGroup.Add(1)
			go runClient(&serverGroup)
			serverGroup.Wait()
			time.Sleep(checkintime)
		}
	}

	waitnetwork.Wait()
}

func runClient(wg *sync.WaitGroup) {
	defer wg.Done()
	go serverctl.HandleContainedClient()
}

func runGRPC(wg *sync.WaitGroup) {

	defer wg.Done()

	grpcport := servercfg.GetGRPCPort()

	listener, err := net.Listen("tcp", ":"+grpcport)
	// Handle errors if any
	if err != nil {
		logger.FatalLog("[netmaker] Unable to listen on port", grpcport, ": error:", err.Error())
	}

	s := grpc.NewServer(
		authServerUnaryInterceptor(),
	)
	// Create NodeService type
	srv := &controller.NodeServiceServer{}

	// Register the service with the server
	nodepb.RegisterNodeServiceServer(s, srv)

	// Start the server in a child routine
	go func() {
		if err := s.Serve(listener); err != nil {
			logger.FatalLog("Failed to serve:", err.Error())
		}
	}()
	logger.Log(0, "Agent Server successfully started on port ", grpcport, "(gRPC)")

	// Right way to stop the server using a SHUTDOWN HOOK
	// Create a channel to receive OS signals
	c := make(chan os.Signal, 1)

	// Relay os.Interrupt to our channel (os.Interrupt = CTRL+C)
	// Ignore other incoming signals
	signal.Notify(c, os.Interrupt)

	// Block main routine until a signal is received
	// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
	<-c

	// After receiving CTRL+C Properly stop the server
	logger.Log(0, "Stopping the Agent server...")
	s.Stop()
	listener.Close()
	logger.Log(0, "Agent server closed..")
	logger.Log(0, "Closed DB connection.")
}

func authServerUnaryInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(controller.AuthServerUnaryInterceptor)
}

func setGarbageCollection() {
	_, gcset := os.LookupEnv("GOGC")
	if !gcset {
		debug.SetGCPercent(ncutils.DEFAULT_GC_PERCENT)
	}
}
