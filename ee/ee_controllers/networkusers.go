package ee_controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
)

func NetworkUsersHandlers(r *mux.Router) {
	r.HandleFunc("/api/networkusers", logic.SecurityCheck(true, http.HandlerFunc(getAllNetworkUsers))).Methods("GET")
	r.HandleFunc("/api/networkusers/{network}", logic.SecurityCheck(true, http.HandlerFunc(getNetworkUsers))).Methods("GET")
	r.HandleFunc("/api/networkusers/{network}/{networkuser}", logic.SecurityCheck(true, http.HandlerFunc(getNetworkUser))).Methods("GET")
	r.HandleFunc("/api/networkusers/{network}", logic.SecurityCheck(true, http.HandlerFunc(createNetworkUser))).Methods("POST")
	r.HandleFunc("/api/networkusers/{network}", logic.SecurityCheck(true, http.HandlerFunc(updateNetworkUser))).Methods("PUT")
	r.HandleFunc("/api/networkusers/data/{networkuser}/me", logic.NetUserSecurityCheck(false, false, http.HandlerFunc(getNetworkUserData))).Methods("GET")
	r.HandleFunc("/api/networkusers/{network}/{networkuser}", logic.SecurityCheck(true, http.HandlerFunc(deleteNetworkUser))).Methods("DELETE")
}

// == RETURN TYPES ==

// NetworkName - represents a network name/ID
type NetworkName string

// NetworkUserDataMap - map of all data per network for a user
type NetworkUserDataMap map[NetworkName]NetworkUserData

// NetworkUserData - data struct for network users
type NetworkUserData struct {
	Nodes    []models.Node         `json:"nodes" bson:"nodes" yaml:"nodes"`
	Clients  []models.ExtClient    `json:"clients" bson:"clients" yaml:"clients"`
	Vpn      []models.Node         `json:"vpns" bson:"vpns" yaml:"vpns"`
	Networks []models.Network      `json:"networks" bson:"networks" yaml:"networks"`
	User     promodels.NetworkUser `json:"user" bson:"user" yaml:"user"`
}

// == END RETURN TYPES ==

// returns a map of a network user's data across all networks
func getNetworkUserData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	networkUserName := params["networkuser"]
	logger.Log(1, r.Header.Get("user"), "requested fetching network user data for user", networkUserName)

	networks, err := logic.GetNetworks()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if networkUserName == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("netuserToGet"), "badrequest"))
		return
	}

	u, err := logic.GetUser(networkUserName)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("could not find user"), "badrequest"))
		return
	}

	// initialize the return data of network users
	returnData := make(NetworkUserDataMap)

	// go through each network and get that user's data
	// if user has no access, give no data
	// if user is a net admin, give all nodes
	// if user has node access, give user's nodes if any
	// if user has client access, git user's clients if any
	for i := range networks {

		netID := networks[i].NetID
		newData := NetworkUserData{
			Nodes:    []models.Node{},
			Clients:  []models.ExtClient{},
			Vpn:      []models.Node{},
			Networks: []models.Network{},
		}
		netUser, err := pro.GetNetworkUser(netID, promodels.NetworkUserID(networkUserName))
		// check if user has access
		if err == nil && netUser.AccessLevel != pro.NO_ACCESS {
			newData.User = promodels.NetworkUser{
				AccessLevel: netUser.AccessLevel,
				ClientLimit: netUser.ClientLimit,
				NodeLimit:   netUser.NodeLimit,
				Nodes:       netUser.Nodes,
				Clients:     netUser.Clients,
			}
			newData.User.SetDefaults()
			// check network level permissions
			if doesNetworkAllow := pro.IsUserAllowed(&networks[i], networkUserName, u.Groups); doesNetworkAllow || netUser.AccessLevel == pro.NET_ADMIN {
				netNodes, err := logic.GetNetworkNodes(netID)
				if err != nil {
					if database.IsEmptyRecord(err) && netUser.AccessLevel == pro.NET_ADMIN {
						newData.Networks = append(newData.Networks, networks[i])
					} else {
						logger.Log(0, "failed to retrieve nodes on network", netID, "for user", string(netUser.ID))
					}
				} else {
					if netUser.AccessLevel <= pro.NODE_ACCESS { // handle nodes
						// if access level is NODE_ACCESS, filter nodes
						if netUser.AccessLevel == pro.NODE_ACCESS {
							for i := range netNodes {
								if logic.StringSliceContains(netUser.Nodes, netNodes[i].ID) {
									newData.Nodes = append(newData.Nodes, netNodes[i])
								}
							}
						} else { // net admin so, get all nodes and ext clients on network...
							newData.Nodes = netNodes
							for i := range netNodes {
								if netNodes[i].IsIngressGateway == "yes" {
									newData.Vpn = append(newData.Vpn, netNodes[i])
									if clients, err := logic.GetExtClientsByID(netNodes[i].ID, netID); err == nil {
										newData.Clients = append(newData.Clients, clients...)
									}
								}
							}
							newData.Networks = append(newData.Networks, networks[i])
						}
					}
					if netUser.AccessLevel <= pro.CLIENT_ACCESS && netUser.AccessLevel != pro.NET_ADMIN {
						for _, c := range netUser.Clients {
							if client, err := logic.GetExtClient(c, netID); err == nil {
								newData.Clients = append(newData.Clients, client)
							}
						}
						for i := range netNodes {
							if netNodes[i].IsIngressGateway == "yes" {
								newData.Vpn = append(newData.Vpn, netNodes[i])
							}
						}
					}
				}
			}
			returnData[NetworkName(netID)] = newData
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(returnData)
}

// returns a map of all network users mapped to each network
func getAllNetworkUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logger.Log(1, r.Header.Get("user"), "requested fetching all network users")
	type allNetworkUsers = map[string][]promodels.NetworkUser

	networks, err := logic.GetNetworks()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	var allNetUsers = make(allNetworkUsers, len(networks))

	for i := range networks {
		netusers, err := pro.GetNetworkUsers(networks[i].NetID)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		for _, v := range netusers {
			allNetUsers[networks[i].NetID] = append(allNetUsers[networks[i].NetID], v)
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(allNetUsers)
}

func getNetworkUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	netname := params["network"]
	logger.Log(1, r.Header.Get("user"), "requested fetching network users for network", netname)

	_, err := logic.GetNetwork(netname)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	netusers, err := pro.GetNetworkUsers(netname)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(netusers)
}

func getNetworkUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	netname := params["network"]
	logger.Log(1, r.Header.Get("user"), "requested fetching network user", params["networkuser"], "on network", netname)

	_, err := logic.GetNetwork(netname)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	netuserToGet := params["networkuser"]
	if netuserToGet == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("netuserToGet"), "badrequest"))
		return
	}

	netuser, err := pro.GetNetworkUser(netname, promodels.NetworkUserID(netuserToGet))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(netuser)
}

func createNetworkUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["network"]
	logger.Log(1, r.Header.Get("user"), "requested creating a network user on network", netname)

	network, err := logic.GetNetwork(netname)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var networkuser promodels.NetworkUser

	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&networkuser)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	err = pro.CreateNetworkUser(&network, &networkuser)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func updateNetworkUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	netname := params["network"]
	logger.Log(1, r.Header.Get("user"), "requested updating a network user on network", netname)

	network, err := logic.GetNetwork(netname)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var networkuser promodels.NetworkUser

	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&networkuser)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if networkuser.ID == "" || !pro.DoesNetworkUserExist(netname, networkuser.ID) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid user "+string(networkuser.ID)), "badrequest"))
		return
	}
	if networkuser.AccessLevel < pro.NET_ADMIN || networkuser.AccessLevel > pro.NO_ACCESS {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid user access level provided"), "badrequest"))
		return
	}

	if networkuser.ClientLimit < 0 || networkuser.NodeLimit < 0 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("negative user limit provided"), "badrequest"))
		return
	}

	u, err := logic.GetUser(string(networkuser.ID))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("invalid user "+string(networkuser.ID)), "badrequest"))
		return
	}

	if !pro.IsUserAllowed(&network, u.UserName, u.Groups) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user must be in allowed groups or users"), "badrequest"))
		return
	}

	if networkuser.AccessLevel == pro.NET_ADMIN {
		currentUser, err := logic.GetUser(string(networkuser.ID))
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user model not found for "+string(networkuser.ID)), "badrequest"))
			return
		}

		if !logic.StringSliceContains(currentUser.Networks, netname) {
			// append network name to user model to conform to old model
			if err = logic.UpdateUserNetworks(
				append(currentUser.Networks, netname),
				currentUser.Groups,
				currentUser.IsAdmin,
				&models.ReturnUser{
					Groups:   currentUser.Groups,
					IsAdmin:  currentUser.IsAdmin,
					Networks: currentUser.Networks,
					UserName: currentUser.UserName,
				},
			); err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user model failed net admin update "+string(networkuser.ID)+" (are they an admin?"), "badrequest"))
				return
			}
		}
	}

	err = pro.UpdateNetworkUser(netname, &networkuser)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteNetworkUser(w http.ResponseWriter, r *http.Request) {

	var params = mux.Vars(r)
	netname := params["network"]

	logger.Log(1, r.Header.Get("user"), "requested deleting network user", params["networkuser"], "on network", netname)

	_, err := logic.GetNetwork(netname)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	netuserToDelete := params["networkuser"]
	if netuserToDelete == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("no group name provided"), "badrequest"))
		return
	}

	if err := pro.DeleteNetworkUser(netname, netuserToDelete); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	w.WriteHeader(http.StatusOK)
}
