//TODO: Harden. Add failover for every method and agent calls
//TODO: Figure out why mongodb keeps failing (log rotation?)

package main

import (
    "log"
    "github.com/gravitl/netmaker/controllers"
    "github.com/gravitl/netmaker/servercfg"
    "github.com/gravitl/netmaker/serverctl"
    "github.com/gravitl/netmaker/mongoconn"
    "github.com/gravitl/netmaker/functions"
    "os"
    "os/exec"
    "net"
    "context"
    "strconv"
    "sync"
    "os/signal"
    service "github.com/gravitl/netmaker/controllers"
    nodepb "github.com/gravitl/netmaker/grpc"
    "google.golang.org/grpc"
)

//Start MongoDB Connection and start API Request Handler
func main() {


	//Client Mode Prereq Check
	if servercfg.IsClientMode() {
		cmd := exec.Command("id", "-u")
		output, err := cmd.Output()

		if err != nil {
			log.Println("Error running 'id -u' for prereq check. Please investigate or disable client mode.")
			log.Fatal(err)
		}
		i, err := strconv.Atoi(string(output[:len(output)-1]))
		if err != nil {
                        log.Println("Error retrieving uid from 'id -u' for prereq check. Please investigate or disable client mode.")
			log.Fatal(err)
		}
		if i != 0 {
			log.Fatal("To run in client mode requires root privileges. Either disable client mode or run with sudo.")
		}
	}
        if servercfg.IsDNSMode() {
                err := functions.SetDNSDir()
                if err != nil {
                        log.Fatal(err)
                }
        }

	//Start Mongodb
	mongoconn.ConnectDatabase()

	installserver := false

	//Create the default network (default: 10.10.10.0/24)
	created, err := serverctl.CreateDefaultNetwork()
	if err != nil {
		log.Printf("Error creating default network: %v", err)
	}

	if created && servercfg.IsClientMode() {
		installserver = true
	}
	if servercfg.IsGRPCWireGuard() {
                err = serverctl.InitServerWireGuard()
		//err = serverctl.ReconfigureServerWireGuard()
	        if err != nil {
	                log.Fatal(err)
		}
	}

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
		go runGRPC(&waitnetwork, installserver)
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


func runGRPC(wg *sync.WaitGroup, installserver bool) {


	defer wg.Done()

        // Configure 'log' package to give file name and line number on eg. log.Fatal
        // Pipe flags to one another (log.LstdFLags = log.Ldate | log.Ltime)
        log.SetFlags(log.LstdFlags | log.Lshortfile)

	grpcport := servercfg.GetGRPCPort()

	listener, err := net.Listen("tcp", ":"+grpcport)
        // Handle errors if any
        if err != nil {
                log.Fatalf("Unable to listen on port " + grpcport + ", error: %v", err)
        }

         s := grpc.NewServer(
		 authServerUnaryInterceptor(),
		 authServerStreamInterceptor(),
	 )
         // Create NodeService type 
         srv := &service.NodeServiceServer{}

         // Register the service with the server 
         nodepb.RegisterNodeServiceServer(s, srv)

         srv.NodeDB = mongoconn.NodeDB

        // Start the server in a child routine
        go func() {
                if err := s.Serve(listener); err != nil {
                        log.Fatalf("Failed to serve: %v", err)
                }
        }()
        log.Println("Agent Server succesfully started on port " + grpcport + " (gRPC)")

	if installserver {
			success := true
			if !servercfg.DisableDefaultNet() {
	                        log.Println("Adding server to default network")
				success, err = serverctl.AddNetwork("default")
			}
			if err != nil {
                                log.Printf("Error adding to default network: %v", err)
				log.Println("Unable to add server to network. Continuing.")
				log.Println("Please investigate client installation on server.")
			} else if !success {
                                log.Println("Unable to add server to network. Continuing.")
                                log.Println("Please investigate client installation on server.")
			} else{
                                log.Println("Server successfully added to default network.")
			}
	}
        log.Println("Setup complete. You are ready to begin using netmaker.")

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
        log.Println("Closing MongoDB connection")
        mongoconn.Client.Disconnect(context.TODO())
        log.Println("MongoDB connection closed.")
}

func authServerUnaryInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(controller.AuthServerUnaryInterceptor)
}
func authServerStreamInterceptor() grpc.ServerOption {
        return grpc.StreamInterceptor(controller.AuthServerStreamInterceptor)
}
