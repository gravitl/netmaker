package mongoconn

import (
	"context"
	"log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
        "github.com/gravitl/netmaker/servercfg"
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
	user = servercfg.GetMongoUser()
	pass = servercfg.GetMongoPass()
	host = servercfg.GetMongoHost()
	port = servercfg.GetMongoPort()
	opts = servercfg.GetMongoOpts()
}

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
