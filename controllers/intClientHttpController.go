package controller

import (
//	"fmt"
	"errors"
	"context"
	"encoding/json"
	"net/http"
	"time"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/serverctl"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func intClientHandlers(r *mux.Router) {

	r.HandleFunc("/api/intclient/{clientid}", securityCheck(http.HandlerFunc(getIntClient))).Methods("GET")
	r.HandleFunc("/api/intclients", securityCheck(http.HandlerFunc(getAllIntClients))).Methods("GET")
        r.HandleFunc("/api/intclients/deleteall", securityCheck(http.HandlerFunc(deleteAllIntClients))).Methods("DELETE")
        r.HandleFunc("/api/intclient/{clientid}", securityCheck(http.HandlerFunc(deleteIntClient))).Methods("DELETE")
        r.HandleFunc("/api/intclient/{clientid}", securityCheck(http.HandlerFunc(updateIntClient))).Methods("PUT")
	r.HandleFunc("/api/intclient/register", http.HandlerFunc(registerIntClient)).Methods("POST")
	r.HandleFunc("/api/intclient/{clientid}", http.HandlerFunc(deleteIntClient)).Methods("DELETE")
}

func getAllIntClients(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        clients, err := functions.GetAllIntClients()
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        //Return all the extclients in JSON format
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(clients)
}

func deleteAllIntClients(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        err := functions.DeleteAllIntClients()
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        w.WriteHeader(http.StatusOK)
}

func deleteIntClient(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        // get params
        var params = mux.Vars(r)

        success, err := DeleteIntClient(params["clientid"])

        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        } else if !success {
                err = errors.New("Could not delete intclient " + params["clientid"])
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        returnSuccessResponse(w, r, params["clientid"]+" deleted.")
}


func getIntClient(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        var params = mux.Vars(r)

	client, err := GetIntClient(params["clientid"])
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(client)
}

func updateIntClient(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        var errorResponse = models.ErrorResponse{
                Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
        }

        var clientreq models.IntClient

        //get node from body of request
        err := json.NewDecoder(r.Body).Decode(&clientreq)
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        if servercfg.IsRegisterKeyRequired() {
                validKey := functions.IsKeyValidGlobal(clientreq.AccessKey)
                if !validKey {
                                errorResponse = models.ErrorResponse{
                                        Code: http.StatusUnauthorized, Message: "W1R3: Key invalid, or none provided.",
                                }
                                returnErrorResponse(w, r, errorResponse)
                                return
                }
        }
        client, err := RegisterIntClient(clientreq)

        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(client)
}

func RegisterIntClient(client models.IntClient) (models.IntClient, error) {
	if client.PrivateKey == "" {
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return client, err
		}

		client.PrivateKey = privateKey.String()
		client.PublicKey = privateKey.PublicKey().String()
	}

	if client.Address == "" {
		newAddress, err := functions.UniqueAddress(client.Network)
		if err != nil {
			return client, err
		}
		if newAddress == "" {
			return client, errors.New("Could not find an address.")
		}
		client.Address = newAddress
	}
        if client.Network == "" { client.Network = "comms" }
	server, err := serverctl.GetServerWGConf()
	if err != nil {
		return client, err
	}
  gcfg := servercfg.GetConfig()
  client.ServerWGEndpoint = server.ServerWGEndpoint
  client.ServerAPIEndpoint = gcfg.APIHost + ":" + gcfg.APIPort
	client.ServerAddress = server.ServerAddress
	client.ServerPort = server.ServerPort
	client.ServerGRPCPort = gcfg.GRPCPort
	client.ServerKey = server.ServerKey

        if client.ClientID == "" {
                clientid := StringWithCharset(7, charset)
                clientname := "client-" + clientid
                client.ClientID = clientname
        }


	collection := mongoconn.Client.Database("netmaker").Collection("intclients")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// insert our network into the network table
	_, err = collection.InsertOne(ctx, client)
	defer cancel()

	if err != nil {
		return client, err
	}

	err = serverctl.ReconfigureServerWireGuard()

	return client, err
}
func registerIntClient(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        var errorResponse = models.ErrorResponse{
                Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
        }

        var clientreq models.IntClient

        //get node from body of request
        err := json.NewDecoder(r.Body).Decode(&clientreq)
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        if servercfg.IsRegisterKeyRequired() {
                validKey := functions.IsKeyValidGlobal(clientreq.AccessKey)
                if !validKey {
                                errorResponse = models.ErrorResponse{
                                        Code: http.StatusUnauthorized, Message: "W1R3: Key invalid, or none provided.",
                                }
                                returnErrorResponse(w, r, errorResponse)
                                return
                }
        }
        client, err := RegisterIntClient(clientreq)

        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(client)
}
