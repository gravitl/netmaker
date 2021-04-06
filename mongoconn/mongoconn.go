package mongoconn

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"net/http"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
        "github.com/gravitl/netmaker/config"
)

var Client *mongo.Client
var NodeDB *mongo.Collection
var GroupDB *mongo.Collection
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
    log.Println("Database connecting...")
    // Set client options

    setVars()

    clientOptions := options.Client().ApplyURI( "mongodb://" +
						user + ":" +
						pass + "@" +
						host + ":" +
						port +
						opts )

    // Connect to MongoDB
    client, err := mongo.Connect(context.TODO(), clientOptions)
    Client = client
    if err != nil {
        log.Fatal(err)
    }

    // Check the connection
    err = Client.Ping(context.TODO(), nil)

    if err != nil {
        log.Fatal(err)
    }

    NodeDB = Client.Database("netmaker").Collection("nodes")
    GroupDB = Client.Database("netmaker").Collection("groups")

    log.Println("Database Connected.")
}

//TODO: IDK if we're using ConnectDB any more.... I think we're just using Client.Database
//Review and see if this is necessary
// ConnectDB : This is helper function to connect mongoDB
func ConnectDB(db string, targetCollection string) *mongo.Collection {

	// Set client options
	//clientOptions := options.Client().ApplyURI("mongodb://mongoadmin:mongopassword@localhost:27017/?authSource=admin")
	clientOptions := options.Client().ApplyURI("mongodb://" + os.Getenv("MONGO_USER") + ":" +
	os.Getenv("MONGO_PASS") + "@" + os.Getenv("MONGO_HOST") + ":" + os.Getenv("MONGO_PORT") + os.Getenv("MONGO_OPTS") )

	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	//collection := client.Database("go_rest_api").Collection("wg")
	collection := client.Database(db).Collection(targetCollection)

	return collection
}

// ErrorResponse : This is error model.
type ErrorResponse struct {
	StatusCode   int    `json:"status"`
	ErrorMessage string `json:"message"`
}

// GetError : This is helper function to prepare error model.
func GetError(err error, w http.ResponseWriter) {

	var response = ErrorResponse{
		ErrorMessage: err.Error(),
		StatusCode:   http.StatusInternalServerError,
	}

	message, _ := json.Marshal(response)

	w.WriteHeader(response.StatusCode)
	w.Write(message)
}
