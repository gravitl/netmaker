//TODO: Harden. Add failover for every method and agent calls
//TODO: Figure out why mongodb keeps failing (log rotation?)

package main

import (
    "log"
    "flag"
    "github.com/gravitl/netmaker/models"
    "github.com/gravitl/netmaker/controllers"
    "github.com/gravitl/netmaker/serverctl"
    "github.com/gravitl/netmaker/functions"
    "github.com/gravitl/netmaker/mongoconn"
    "github.com/gravitl/netmaker/config"
    "go.mongodb.org/mongo-driver/bson"
    "fmt"
    "time"
    "net/http"
    "strings"
    "errors"
    "io/ioutil"
    "os"
    "os/exec"
    "net"
    "context"
    "strconv"
    "sync"
    "os/signal"
    "go.mongodb.org/mongo-driver/mongo"
    service "github.com/gravitl/netmaker/controllers"
    nodepb "github.com/gravitl/netmaker/grpc"
    "google.golang.org/grpc"
)

var ServerGRPC string
var PortGRPC string

//Start MongoDB Connection and start API Request Handler
func main() {

	var clientmode string
	var defaultnet string
	flag.StringVar(&clientmode, "clientmode", "on", "Have a client on the server")
	flag.StringVar(&defaultnet, "defaultnet", "on", "Create a default network")
	flag.Parse()
	if clientmode == "on" {

         cmd := exec.Command("id", "-u")
         output, err := cmd.Output()

         if err != nil {
                 log.Fatal(err)
         }
         i, err := strconv.Atoi(string(output[:len(output)-1]))
         if err != nil {
                 log.Fatal(err)
         }

         if i != 0 {
                 log.Fatal("To run in client mode requires root privileges. Either turn off client mode with the --clientmode=off flag, or run with sudo.")
         }
	}

	log.Println("Server starting...")
	mongoconn.ConnectDatabase()

	installserver := false
	if !(defaultnet == "off") {
	if config.Config.Server.CreateDefault {
		created, err := createDefaultNetwork()
		if err != nil {
			fmt.Printf("Error creating default network: %v", err)
		}
		if created && clientmode != "off" {
			installserver = true
		}
	}
	}
	var waitnetwork sync.WaitGroup

	if config.Config.Server.AgentBackend {
		waitnetwork.Add(1)
		go runGRPC(&waitnetwork, installserver)
	}

	if config.Config.Server.RestBackend {
		waitnetwork.Add(1)
		controller.HandleRESTRequests(&waitnetwork)
	}
	if !config.Config.Server.RestBackend && !config.Config.Server.AgentBackend {
		fmt.Println("Oops! No Server Mode selected. Nothing being served.")
	}
	waitnetwork.Wait()
	fmt.Println("Exiting now.")
}


func runGRPC(wg *sync.WaitGroup, installserver bool) {


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

	if err != nil && err != mongo.ErrNoDocuments{
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

	if installserver {
			fmt.Println("Adding server to " + config.Config.Server.DefaultNetName)
                        success, err := serverctl.AddNetwork(config.Config.Server.DefaultNetName)
                        if err != nil || !success {
                                fmt.Printf("Error adding to default network: %v", err)
				fmt.Println("")
				fmt.Println("Unable to add server to network. Continuing.")
				fmt.Println("Please investigate client installation on server.")
			} else {
                                fmt.Println("Server successfully added to default network.")
			}
	}
        fmt.Println("Setup complete. You are ready to begin using netmaker.")



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

	create, _, err := functions.GetGlobalConfig()
	if create {
		_, err := collection.InsertOne(ctx, globalconf)
		defer cancel()
		if err != nil {
			if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "no documents in result"){
				return nil
			} else {
				return err
			}
		}
	} else {
		filter := bson.M{"name": "netmaker"}
		update := bson.D{
			{"$set", bson.D{
				{"servergrpc", globalconf.ServerGRPC},
				{"portgrpc", globalconf.PortGRPC},
			}},
		}
		err := collection.FindOneAndUpdate(ctx, filter, update).Decode(&globalconf)
                        if err == mongo.ErrNoDocuments {
			//if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "no documents in result"){
                                return nil
                        }
	}
	return err
}

func createDefaultNetwork() (bool, error) {

	iscreated := false
	exists, err := functions.NetworkExists(config.Config.Server.DefaultNetName)

	if exists || err != nil {
		fmt.Println("Default network already exists")
		fmt.Println("Skipping default network create")
		return iscreated, err
	} else {

	var network models.Network

	network.NetID = config.Config.Server.DefaultNetName
	network.AddressRange = config.Config.Server.DefaultNetRange
	network.DisplayName = config.Config.Server.DefaultNetName
        network.SetDefaults()
        network.SetNodesLastModified()
        network.SetNetworkLastModified()
        network.KeyUpdateTimeStamp = time.Now().Unix()
	priv := false
	network.IsLocal = &priv
        network.KeyUpdateTimeStamp = time.Now().Unix()
	allow := true
	network.AllowManualSignUp = &allow

	fmt.Println("Creating default network.")


        collection := mongoconn.Client.Database("netmaker").Collection("networks")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)


        // insert our network into the network table
        _, err = collection.InsertOne(ctx, network)
        defer cancel()

	}
	if err == nil {
		iscreated = true
	}
	return iscreated, err


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

