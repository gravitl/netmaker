//TODO: Harden. Add failover for every method and agent calls
//TODO: Figure out why mongodb keeps failing (log rotation?)

package main

import (
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"

	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
	"google.golang.org/grpc"
)

//Start MongoDB Connection and start API Request Handler
func main() {
	checkModes() // check which flags are set and if root or not
	initialize() // initial db and grpc server
	defer database.Database.Close()
	startControllers() // start the grpc or rest endpoints
}

func checkModes() { // Client Mode Prereq Check
	var err error
	cmd := exec.Command("id", "-u")
	output, err := cmd.Output()

	if err != nil {
		log.Println("Error running 'id -u' for prereq check. Please investigate or disable client mode.")
		log.Fatal(err)
	}
	uid, err := strconv.Atoi(string(output[:len(output)-1]))
	if err != nil {
		log.Println("Error retrieving uid from 'id -u' for prereq check. Please investigate or disable client mode.")
		log.Fatal(err)
	}
	if uid != 0 {
		log.Fatal("To run in client mode requires root privileges. Either disable client mode or run with sudo.")
	}

	if servercfg.IsDNSMode() {
		err := functions.SetDNSDir()
		if err != nil {
			log.Fatal(err)
		}
	}

}

func initialize() {
	database.InitializeDatabase()
	if servercfg.IsGRPCWireGuard() {
		if err := serverctl.InitServerWireGuard(); err != nil {
			log.Fatal(err)
		}
	}
	functions.PrintUserLog("netmaker", "successfully created db tables if not present", 1)
}

func startControllers() {
	var waitnetwork sync.WaitGroup
	//Run Agent Server
	if servercfg.IsAgentBackend() {
		if !(servercfg.DisableRemoteIPCheck()) && servercfg.GetGRPCHost() == "127.0.0.1" {
			err := servercfg.SetHost()
			if err != nil {
				log.Println("Unable to Set host. Exiting...")
				log.Fatal(err)
			}
		}
		waitnetwork.Add(1)
		go runGRPC(&waitnetwork)
	}
	if servercfg.IsDNSMode() {
		err := controller.SetDNS()
		if err != nil {
			log.Fatal(err)
		}
	}
	//Run Rest Server
	if servercfg.IsRestBackend() {
		if !servercfg.DisableRemoteIPCheck() && servercfg.GetAPIHost() == "127.0.0.1" {
			err := servercfg.SetHost()
			if err != nil {
				log.Println("Unable to Set host. Exiting...")
				log.Fatal(err)
			}
		}
		waitnetwork.Add(1)
		controller.HandleRESTRequests(&waitnetwork)
	}
	if !servercfg.IsAgentBackend() && !servercfg.IsRestBackend() {
		log.Println("No Server Mode selected, so nothing is being served! Set either Agent mode (AGENT_BACKEND) or Rest mode (REST_BACKEND) to 'true'.")
	}
	waitnetwork.Wait()
	log.Println("exiting")
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
		log.Fatalf("Unable to listen on port "+grpcport+", error: %v", err)
	}

	s := grpc.NewServer(
		authServerUnaryInterceptor(),
		authServerStreamInterceptor(),
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
	log.Println("Agent Server succesfully started on port " + grpcport + " (gRPC)")

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
	log.Println("Stopping the Agent server...")
	s.Stop()
	listener.Close()
	log.Println("Agent server closed..")
	log.Println("Closed DB connection.")
}

func authServerUnaryInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(controller.AuthServerUnaryInterceptor)
}
func authServerStreamInterceptor() grpc.ServerOption {
	return grpc.StreamInterceptor(controller.AuthServerStreamInterceptor)
}
