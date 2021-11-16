//TODO: Harden. Add failover for every method and agent calls
//TODO: Figure out why mongodb keeps failing (log rotation?)

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/gravitl/netmaker/auth"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	nodepb "github.com/gravitl/netmaker/grpc"
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
	defer database.CloseDB()
	startControllers() // start the grpc or rest endpoints
}

func initialize() { // Client Mode Prereq Check
	var err error

	if err = database.InitializeDatabase(); err != nil {
		logic.Log("Error connecting to database", 0)
		log.Fatal(err)
	}
	logic.Log("database successfully connected", 0)

	var authProvider = auth.InitializeAuthProvider()
	if authProvider != "" {
		logic.Log("OAuth provider, "+authProvider+", initialized", 0)
	} else {
		logic.Log("no OAuth provider found or not configured, continuing without OAuth", 0)
	}

	if servercfg.IsClientMode() != "off" {
		output, err := ncutils.RunCmd("id -u", true)
		if err != nil {
			logic.Log("Error running 'id -u' for prereq check. Please investigate or disable client mode.", 0)
			log.Fatal(output, err)
		}
		uid, err := strconv.Atoi(string(output[:len(output)-1]))
		if err != nil {
			logic.Log("Error retrieving uid from 'id -u' for prereq check. Please investigate or disable client mode.", 0)
			log.Fatal(err)
		}
		if uid != 0 {
			log.Fatal("To run in client mode requires root privileges. Either disable client mode or run with sudo.")
		}
		if err := serverctl.InitServerNetclient(); err != nil {
			log.Fatal("Did not find netclient to use CLIENT_MODE")
		}
	}

	if servercfg.IsDNSMode() {
		err := functions.SetDNSDir()
		if err != nil {
			log.Fatal(err)
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
				logic.Log("Unable to Set host. Exiting...", 0)
				log.Fatal(err)
			}
		}
		waitnetwork.Add(1)
		go runGRPC(&waitnetwork)
	}

	if servercfg.IsClientMode() == "on" {
		waitnetwork.Add(1)
		go runClient(&waitnetwork)
	}

	if servercfg.IsDNSMode() {
		err := logic.SetDNS()
		if err != nil {
			logic.Log("error occurred initializing DNS: "+err.Error(), 0)
		}
	}
	//Run Rest Server
	if servercfg.IsRestBackend() {
		if !servercfg.DisableRemoteIPCheck() && servercfg.GetAPIHost() == "127.0.0.1" {
			err := servercfg.SetHost()
			if err != nil {
				logic.Log("Unable to Set host. Exiting...", 0)
				log.Fatal(err)
			}
		}
		waitnetwork.Add(1)
		controller.HandleRESTRequests(&waitnetwork)
	}
	if !servercfg.IsAgentBackend() && !servercfg.IsRestBackend() {
		logic.Log("No Server Mode selected, so nothing is being served! Set either Agent mode (AGENT_BACKEND) or Rest mode (REST_BACKEND) to 'true'.", 0)
	}

	waitnetwork.Wait()
	logic.Log("exiting", 0)
}

func runClient(wg *sync.WaitGroup) {
	defer wg.Done()
	go func() {
		for {
			if err := serverctl.HandleContainedClient(); err != nil {
				// PASS
			}
			var checkintime = time.Duration(servercfg.GetServerCheckinInterval()) * time.Second
			time.Sleep(checkintime)
		}
	}()
}

func runGRPC(wg *sync.WaitGroup) {

	defer wg.Done()

	// Configure 'log' package to give file name and line number on eg. log.Fatal
	// Pipe flags to one another (log.LstdFLags = log.Ldate | log.Ltime)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	grpcport := servercfg.GetGRPCPort()

	listener, err := net.Listen("tcp", ":"+grpcport)
	// Handle errors if any
	if err != nil {
		log.Fatalf("[netmaker] Unable to listen on port "+grpcport+", error: %v", err)
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
			log.Fatalf("Failed to serve: %v", err)
		}
	}()
	logic.Log("Agent Server successfully started on port "+grpcport+" (gRPC)", 0)

	// Right way to stop the server using a SHUTDOWN HOOK
	// Create a channel to receive OS signals
	c := make(chan os.Signal)

	// Relay os.Interrupt to our channel (os.Interrupt = CTRL+C)
	// Ignore other incoming signals
	signal.Notify(c, os.Interrupt)

	// Block main routine until a signal is received
	// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
	<-c

	// After receiving CTRL+C Properly stop the server
	logic.Log("Stopping the Agent server...", 0)
	s.Stop()
	listener.Close()
	logic.Log("Agent server closed..", 0)
	logic.Log("Closed DB connection.", 0)
}

func authServerUnaryInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(controller.AuthServerUnaryInterceptor)
}

// func authServerStreamInterceptor() grpc.ServerOption {
// 	return grpc.StreamInterceptor(controller.AuthServerStreamInterceptor)
// }
