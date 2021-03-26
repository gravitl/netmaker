//TODO: Harden. Add failover for every method and agent calls
//TODO: Figure out why mongodb keeps failing (log rotation?)

package main

import (
    "log"
    "github.com/gravitl/netmaker/controllers"
    "github.com/gravitl/netmaker/mongoconn"
    "github.com/gravitl/netmaker/config"
    "fmt"
    "os"
    "net"
    "context"
    "sync"
    "os/signal"
    service "github.com/gravitl/netmaker/controllers"
    nodepb "github.com/gravitl/netmaker/grpc"
    "google.golang.org/grpc"
)
//Start MongoDB Connection and start API Request Handler
func main() {
	log.Println("Server starting...")
	mongoconn.ConnectDatabase()

	var waitgroup sync.WaitGroup

	if config.Config.Server.AgentBackend {
		waitgroup.Add(1)
		go runGRPC(&waitgroup)
	}

	if config.Config.Server.RestBackend {
		waitgroup.Add(1)
		controller.HandleRESTRequests(&waitgroup)
	}
	if !config.Config.Server.RestBackend && !config.Config.Server.AgentBackend {
		fmt.Println("Oops! No Server Mode selected. Nothing being served.")
	}
	waitgroup.Wait()
	fmt.Println("Exiting now.")
}


func runGRPC(wg *sync.WaitGroup) {


	defer wg.Done()

        // Configure 'log' package to give file name and line number on eg. log.Fatal
        // Pipe flags to one another (log.LstdFLags = log.Ldate | log.Ltime)
        log.SetFlags(log.LstdFlags | log.Lshortfile)

        // Start our listener, 50051 is the default gRPC port
	grpcport := ":50051"
	if config.Config.Server.GrpcPort != "" {
		grpcport = ":" + config.Config.Server.GrpcPort
	}
        if os.Getenv("GRPC_PORT") != "" {
		grpcport = ":" + os.Getenv("GRPC_PORT")
        }


	listener, err := net.Listen("tcp", grpcport)
        // Handle errors if any
        if err != nil {
                log.Fatalf("Unable to listen on port" + grpcport + ": %v", err)
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
        fmt.Println("Agent Server succesfully started on port " + grpcport + " (gRPC)")

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
        fmt.Println("Stopping the Agent server...")
        s.Stop()
        listener.Close()
        fmt.Println("Agent server closed..")
        fmt.Println("Closing MongoDB connection")
        mongoconn.Client.Disconnect(context.TODO())
        fmt.Println("MongoDB connection closed.")
}

func authServerUnaryInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(controller.AuthServerUnaryInterceptor)
}
func authServerStreamInterceptor() grpc.ServerOption {
        return grpc.StreamInterceptor(controller.AuthServerStreamInterceptor)
}

