package controller

import (
	"context"
	"encoding/json"
	"fmt"

	// "fmt"
	"net/http"
	"time"
	"strconv"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/serverctl"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"github.com/skip2/go-qrcode"
)

func intClientHandlers(r *mux.Router) {

	r.HandleFunc("/api/wgconf/{macaddress}", securityCheck(http.HandlerFunc(getWGClientConf))).Methods("GET")
	r.HandleFunc("/api/register", securityCheck(http.HandlerFunc(registerClient))).Methods("POST")
}

//Get an individual extclient. Nothin fancy here folks.
func getWGClientConf(w http.ResponseWriter, r *http.Request) {
        // set header.
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var extclient models.ExtClient

        collection := mongoconn.Client.Database("netmaker").Collection("extclients")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"network": "grpc", "clientid": params["clientid"]}
        err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&extclient)
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }

        gwnode, err := functions.GetNodeByMacAddress(extclient.Network, extclient.IngressGatewayID)
        if err != nil {
		fmt.Println("Could not retrieve Ingress Gateway Node " + extclient.IngressGatewayID)
		returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }

	network, err := functions.GetParentNetwork(extclient.Network)
        if err != nil {
                fmt.Println("Could not retrieve Ingress Gateway Network " + extclient.Network)
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
	keepalive := ""
	if network.DefaultKeepalive != 0 {
		keepalive = "PersistentKeepalive = " + strconv.Itoa(int(network.DefaultKeepalive))
	}
	gwendpoint := gwnode.Endpoint + ":" + strconv.Itoa(int(gwnode.ListenPort))
	config := fmt.Sprintf(`[Interface]
Address = %s
PrivateKey = %s

[Peer]
PublicKey = %s
AllowedIPs = %s
Endpoint = %s
%s

`, extclient.Address + "/32",
   extclient.PrivateKey,
   gwnode.PublicKey,
   network.AddressRange,
   gwendpoint,
   keepalive)

	if params["type"] == "qr" {
		bytes, err := qrcode.Encode(config, qrcode.Medium, 220)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(bytes)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
		return
	}

	if params["type"] == "file" {
		name := extclient.ClientID + ".conf"
                w.Header().Set("Content-Type", "application/config")
		w.Header().Set("Content-Disposition", "attachment; filename=\"" + name + "\"")
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(w, config)
		if err != nil {
                        returnErrorResponse(w, r, formatError(err, "internal"))
		}
		return
	}

        defer cancel()

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(extclient)
}

func RegisterClient(client models.IntClient) (models.IntClient, error) {
	if client.PrivateKey == "" {
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return client, err
		}

		client.PrivateKey = privateKey.String()
		client.PublicKey = privateKey.PublicKey().String()
	}

	if client.Address == "" {
		newAddress, err := functions.UniqueAddress6(client.Network)
		if err != nil {
			return client, err
		}
		client.Address6 = newAddress
	}
        if client.Network == "" { client.Network = "comms" }
	server, err := serverctl.GetServerWGConf()
	if err != nil {
		return client, err
	}
	client.ServerEndpoint = server.ServerEndpoint
	client.ServerAddress = server.ServerAddress
	client.ServerPort = server.ServerPort
	client.ServerKey = server.ServerKey


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
func registerClient(w http.ResponseWriter, r *http.Request) {
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
        client, err := RegisterClient(clientreq)

        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(client)
}
