package mongoconn

import (
	"context"
	"log"
	"os"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
        "github.com/gravitl/netmaker/config"
)

var Client *mongo.Client
var NodeDB *mongo.Collection
var NetworkDB *mongo.Collection
var user string
var pass string
var host string
var port string
var opts string

func setVars() {

	//defaults
	user = "admin"
	pass = "password"
	host = "localhost"
	port = "27017"
	opts = "/?authSource=admin"

	//override with settings from config file
	if config.Config.MongoConn.User != "" {
		user = config.Config.MongoConn.User
	}
        if config.Config.MongoConn.Pass != "" {
                pass = config.Config.MongoConn.Pass
        }
        if config.Config.MongoConn.Host != "" {
                host = config.Config.MongoConn.Host
        }
        if config.Config.MongoConn.Port != "" {
                port = config.Config.MongoConn.Port
        }
        if config.Config.MongoConn.Opts != "" {
                opts = config.Config.MongoConn.Opts
        }

	//override with settings from env
	if os.Getenv("MONGO_USER") != "" {
		user = os.Getenv("MONGO_USER")
	}
        if os.Getenv("MONGO_PASS") != "" {
                pass = os.Getenv("MONGO_PASS")
        }
        if os.Getenv("MONGO_HOST") != "" {
                host = os.Getenv("MONGO_HOST")
        }
        if os.Getenv("MONGO_PORT") != "" {
                port = os.Getenv("MONGO_PORT")
        }
        if os.Getenv("MONGO_OPTS") != "" {
                opts = os.Getenv("MONGO_OPTS")
        }
}

//TODO: are we  even using  this besides at startup? Is it truely necessary?
//TODO: Use config file instead of os.Getenv
func ConnectDatabase() {
    // Set client options

    setVars()

    clientOptions := options.Client().ApplyURI( "mongodb://" +
						user + ":" +
						pass + "@" +
						host + ":" +
						port +
						opts )

    // Connect to MongoDB
    log.Println("Connecting to MongoDB at " + host + ":" + port + "...")
    client, err := mongo.Connect(context.TODO(), clientOptions)
    Client = client
    if err != nil {
	log.Println("Error encountered connecting to MongoDB. Terminating.")
        log.Fatal(err)
    }

    // Check the connection
    err = Client.Ping(context.TODO(), nil)

    if err != nil {
	log.Println("Error encountered pinging MongoDB. Terminating.")
        log.Fatal(err)
    }

    NodeDB = Client.Database("netmaker").Collection("nodes")
    NetworkDB = Client.Database("netmaker").Collection("networks")

    log.Println("MongoDB Connected.")
}

// ErrorResponse : This is error model.
type ErrorResponse struct {
	StatusCode   int    `json:"status"`
	ErrorMessage string `json:"message"`
}
