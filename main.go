//TODO: Harden. Add failover for every method and agent calls
//TODO: Figure out why mongodb keeps failing (log rotation?)

package main

import (
    "log"
    "github.com/gravitl/netmaker/models"
    "github.com/gravitl/netmaker/controllers"
    "github.com/gravitl/netmaker/functions"
    "github.com/gravitl/netmaker/mongoconn"
    "github.com/gravitl/netmaker/config"
    "go.mongodb.org/mongo-driver/bson"
    "fmt"
    "time"
    "net/http"
    "errors"
    "io/ioutil"
    "os"
    "net"
    "context"
    "sync"
    "os/signal"
    service "github.com/gravitl/netmaker/controllers"
    nodepb "github.com/gravitl/netmaker/grpc"
    "google.golang.org/grpc"
)

var ServerGRPC string
var PortGRPC string

//Start MongoDB Connection and start API Request Handler
func main() {
	log.Println("Server starting...")
	mongoconn.ConnectDatabase()

	if config.Config.Server.CreateDefault {
		err := createDefaultNetwork()
		if err != nil {
			fmt.Printf("Error creating default network: %v", err)
		}
	}

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
	PortGRPC = grpcport
	if os.Getenv("BACKEND_URL") == ""  {
		if config.Config.Server.Host == "" {
			ServerGRPC, _ = getPublicIP()
		} else {
			ServerGRPC = config.Config.Server.Host
		}
	} else {
		ServerGRPC = os.Getenv("BACKEND_URL")
	}
	fmt.Println("GRPC Server set to: " + ServerGRPC)
	fmt.Println("GRPC Port set to: " + PortGRPC)
	var gconf models.GlobalConfig
	gconf.ServerGRPC = ServerGRPC
	gconf.PortGRPC = PortGRPC
	gconf.Name = "netmaker"
	err := setGlobalConfig(gconf)

	if err != nil {
	      log.Fatalf("Unable to set global config: %v", err)
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
func setGlobalConfig(globalconf models.GlobalConfig) (error) {

        collection := mongoconn.Client.Database("netmaker").Collection("config")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	_, err := functions.GetGlobalConfig()
	if err != nil {
		_, err := collection.InsertOne(ctx, globalconf)
		defer cancel()
		if err != nil {
			return err
		}
	} else {
		filter := bson.M{"name": "netmaker"}
		update := bson.D{
			{"$set", bson.D{
				{"servergrpc", globalconf.ServerGRPC},
				{"portgrpc", globalconf.PortGRPC},
			}},
		}
		err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&globalconf)
	}
	return nil
}

func createDefaultNetwork() error {


	exists, err := functions.GroupExists(config.Config.Server.DefaultNetName)

	if exists || err != nil {
		fmt.Println("Default group already exists")
		fmt.Println("Skipping default group create")
		return err
	} else {

	var group models.Group

	group.NameID = config.Config.Server.DefaultNetName
	group.AddressRange = config.Config.Server.DefaultNetRange
	group.DisplayName = config.Config.Server.DefaultNetName
        group.SetDefaults()
        group.SetNodesLastModified()
        group.SetGroupLastModified()
        group.KeyUpdateTimeStamp = time.Now().Unix()


	fmt.Println("Creating default group.")


        collection := mongoconn.Client.Database("netmaker").Collection("groups")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)


        // insert our group into the group table
        _, err = collection.InsertOne(ctx, group)
        defer cancel()

	}
	return err


}


func getPublicIP() (string, error) {

        iplist := []string{"https://ifconfig.me", "http://api.ipify.org", "http://ipinfo.io/ip"}
        endpoint := ""
        var err error
            for _, ipserver := range iplist {
                resp, err := http.Get(ipserver)
                if err != nil {
                        continue
                }
                defer resp.Body.Close()
                if resp.StatusCode == http.StatusOK {
                        bodyBytes, err := ioutil.ReadAll(resp.Body)
                        if err != nil {
                                continue
                        }
                        endpoint = string(bodyBytes)
                        break
                }

        }
        if err == nil && endpoint == "" {
                err =  errors.New("Public Address Not Found.")
        }
        return endpoint, err
}


func authServerUnaryInterceptor() grpc.ServerOption {
	return grpc.UnaryInterceptor(controller.AuthServerUnaryInterceptor)
}
func authServerStreamInterceptor() grpc.ServerOption {
        return grpc.StreamInterceptor(controller.AuthServerStreamInterceptor)
}

