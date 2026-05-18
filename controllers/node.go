package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/db/expr"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

var hostIDHeader = "host-id"

func nodeHandlers(r *mux.Router) {

	r.HandleFunc("/api/nodes", logic.SecurityCheck(true, http.HandlerFunc(getAllNodes))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}", logic.SecurityCheck(true, http.HandlerFunc(getNetworkNodes))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/nodes/{network}", logic.SecurityCheck(true, http.HandlerFunc(listNetworkNodes))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", AuthorizeHost(http.HandlerFunc(getNode))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", logic.SecurityCheck(true, http.HandlerFunc(updateNode))).Methods(http.MethodPut)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", AuthorizeHost(http.HandlerFunc(deleteNode))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/creategateway", logic.SecurityCheck(true, http.HandlerFunc(createEgressGateway))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deletegateway", logic.SecurityCheck(true, http.HandlerFunc(deleteEgressGateway))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/createingress", logic.SecurityCheck(true, http.HandlerFunc(createGateway))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deleteingress", logic.SecurityCheck(true, http.HandlerFunc(deleteGateway))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/adm/{network}/authenticate", authenticate).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/nodes/{network}/bulk", logic.SecurityCheck(true, http.HandlerFunc(bulkDeleteNodes))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/nodes/{network}/bulk/status", logic.SecurityCheck(true, http.HandlerFunc(bulkUpdateNodeStatus))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/nodes/{network}/status", logic.SecurityCheck(true, http.HandlerFunc(getNetworkNodeStatus))).Methods(http.MethodGet)
}

func authenticate(response http.ResponseWriter, request *http.Request) {

	var authRequest models.AuthParams
	var result models.Node
	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	decoder := json.NewDecoder(request.Body)
	decoderErr := decoder.Decode(&authRequest)
	defer request.Body.Close()

	if decoderErr != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = decoderErr.Error()
		logger.Log(0, request.Header.Get("user"), "error decoding request body: ",
			decoderErr.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	errorResponse.Code = http.StatusBadRequest
	if authRequest.ID == "" {
		errorResponse.Message = "W1R3: ID can't be empty"
		logger.Log(0, request.Header.Get("user"), errorResponse.Message)
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	} else if authRequest.Password == "" {
		errorResponse.Message = "W1R3: Password can't be empty"
		logger.Log(0, request.Header.Get("user"), errorResponse.Message)
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	var err error
	result, err = logic.GetNodeByID(authRequest.ID)
	if err != nil {
		result, err = logic.GetDeletedNodeByID(authRequest.ID)
		if err != nil {
			errorResponse.Code = http.StatusBadRequest
			errorResponse.Message = err.Error()
			logger.Log(0, request.Header.Get("user"),
				fmt.Sprintf("failed to get node info [%s]: %v", authRequest.ID, err))
			logic.ReturnErrorResponse(response, request, errorResponse)
			return
		}
	}
	host := &schema.Host{
		ID: result.HostID,
	}
	err = host.Get(request.Context())
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error retrieving host: ", result.HostID.String(), err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(host.HostPass), []byte(authRequest.Password))
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error validating user password: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	tokenString, err := logic.CreateJWT(authRequest.ID, authRequest.MacAddress, result.Network)
	if tokenString == "" {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = "Could not create Token"
		logger.Log(0, request.Header.Get("user"),
			fmt.Sprintf("%s: %v", errorResponse.Message, err))
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	var successResponse = models.SuccessResponse{
		Code:    http.StatusOK,
		Message: "W1R3: Device " + authRequest.ID + " Authorized",
		Response: models.SuccessfulLoginResponse{
			AuthToken: tokenString,
			ID:        authRequest.ID,
		},
	}
	successJSONResponse, jsonError := json.Marshal(successResponse)

	if jsonError != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error marshalling resp: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	response.WriteHeader(http.StatusOK)
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

// AuthorizeHost - middleware that authenticates a host via JWT and ensures
// the host is only operating on its own resources (matched by hostid/nodeid path params).
func AuthorizeHost(
	next http.Handler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var forbiddenResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: logic.Forbidden_Msg,
		}
		w.Header().Set("Content-Type", "application/json")

		bearerToken := r.Header.Get("Authorization")
		var tokenSplit = strings.Split(bearerToken, " ")
		var authToken = ""

		if len(tokenSplit) < 2 {
			logic.ReturnErrorResponse(w, r, logic.FormatError(logic.Unauthorized_Err, logic.Unauthorized_Msg))
			return
		} else {
			authToken = tokenSplit[1]
		}

		hostID, _, _, err := logic.VerifyHostToken(authToken)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(logic.Unauthorized_Err, logic.Unauthorized_Msg))
			return
		}

		// master key bypasses ownership checks
		if hostID != logic.MasterUser {
			params := mux.Vars(r)
			if paramHostID := params["hostid"]; paramHostID != "" && hostID != paramHostID {
				logic.ReturnErrorResponse(w, r, forbiddenResponse)
				return
			}
			if nodeID := params["nodeid"]; nodeID != "" {
				node, err := logic.GetNodeByID(nodeID)
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
					return
				}
				if node.HostID.String() != hostID {
					logic.ReturnErrorResponse(w, r, forbiddenResponse)
					return
				}
			}
		}
		r.Header.Set(hostIDHeader, hostID)
		next.ServeHTTP(w, r)
	}
}

// @Summary     List all nodes in the network
// @Router      /api/v1/nodes/{network} [get]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       os query []string false "Filter by OS" Enums(windows, linux, darwin)
// @Param       status query []string false "Filter by Status" Enums(offline, online, disconnected, warning, error)
// @Param       device_type query string false "Filter by Device Type" Enums(gw, igw, gw_assigned, gw_unassigned)
// @Param       q query string false "Search across fields"
// @Param       page query int false "Page number"
// @Param       per_page query int false "Items per page"
// @Success     200 {array} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func listNetworkNodes(w http.ResponseWriter, r *http.Request) {
	networkName := mux.Vars(r)["network"]

	var osFilters []interface{}
	for _, filter := range r.URL.Query()["os"] {
		osFilters = append(osFilters, filter)
	}

	deviceType := r.URL.Query().Get("device_type")

	var statusFilters []interface{}
	for _, filter := range r.URL.Query()["status"] {
		statusFilters = append(statusFilters, filter)
	}

	q := r.URL.Query().Get("q")

	var page, pageSize int
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page == 0 {
		page = 1
	}

	pageSize, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	network := &schema.Network{
		Name: networkName,
	}
	err := network.Get(r.Context())
	if err != nil {
		errType := logic.Internal
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errType = logic.BadReq
		}

		err = fmt.Errorf("failed to fetch nodes in network %s: error fetching network: %v", networkName, err)
		logger.Log(0, r.Header.Get("user"), err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}

	var filters, options []dbtypes.Option
	filters = append(filters, dbtypes.WithFilter("network_id", network.ID))
	filters = append(filters, dbtypes.WithJoin("Host", dbtypes.WithFilter("os", osFilters...)))
	filters = append(filters, dbtypes.WithFilter("status", statusFilters...))

	if deviceType != "" {
		switch deviceType {
		case "gw":
			filters = append(filters, dbtypes.WithFilter("is_gateway", true))
		case "igw":
			filters = append(filters, dbtypes.WithFilter("is_internet_gateway", true))
		case "gw_assigned":
			filters = append(filters, dbtypes.WithNotFilter("relayed_by_node_id", nil))
		case "gw_unassigned":
			filters = append(filters, dbtypes.WithFilter("relayed_by_node_id", nil))
		}
	}

	filters = append(filters, dbtypes.WithSearchQuery(
		q,
		fmt.Sprintf("%s.id", (&schema.Node{}).TableName()),
		"name",
		"address",
		"address6",
		expr.ByteaField("endpoint_ip"),
		expr.ByteaField("endpoint_ipv6"),
	))
	options = append(options, filters...)
	options = append(options, dbtypes.InAscOrder(fmt.Sprintf("%s.created_at", (&schema.Node{}).TableName())))
	options = append(options, dbtypes.WithPagination(page, pageSize))

	_nodes, err := (&schema.Node{}).ListAll(r.Context(), options...)
	if err != nil {
		err = fmt.Errorf("failed to fetch nodes in network %s: error fetching nodes: %v", networkName, err)
		logger.Log(0, r.Header.Get("user"), err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	nodes := make([]models.NodeWithHost, 0, len(_nodes))
	for _, _node := range _nodes {
		var node models.NodeWithHost
		node.Fill(&_node)
		nodes = append(nodes, node)
	}

	logger.Log(2, r.Header.Get("user"), "fetched nodes in network", networkName)

	total, err := (&schema.Node{}).Count(r.Context(), filters...)
	if err != nil {
		err = fmt.Errorf("failed to fetch nodes in network %s: error constructing page: %v", networkName, err)
		logger.Log(0, r.Header.Get("user"), err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := models.PaginatedResponse{
		Data:       nodes,
		Page:       page,
		PerPage:    pageSize,
		Total:      total,
		TotalPages: totalPages,
	}

	logic.ReturnSuccessResponseWithJson(w, r, response, "fetched network nodes")
}

// @Summary     Gets all nodes associated with network including pending nodes
// @Router      /api/nodes/{network} [get]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Success     200 {array} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func getNetworkNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	networkName := params["network"]
	nodes, err := logic.GetNetworkNodes(networkName)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching nodes on network %s: %v", networkName, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	nodes = logic.AddStaticNodestoList(nodes)
	// returns all the nodes in JSON/API format
	apiNodes := logic.GetAllNodesAPI(nodes[:])
	for i := range apiNodes {
		apiNodes[i].StaticNode.PrivateKey = ""
	}
	logger.Log(2, r.Header.Get("user"), "fetched nodes on network", networkName)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNodes)
}

// @Summary     Get all nodes across all networks
// @Router      /api/nodes [get]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Success     200 {array} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func getAllNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var nodes []models.Node
	nodes, err := logic.GetAllNodes()
	if err != nil {
		logger.Log(0, "error fetching all nodes info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	username := r.Header.Get("user")
	if r.Header.Get("ismaster") == "no" {
		user := &schema.User{Username: username}
		err = user.Get(r.Context())
		if err != nil {
			return
		}
		userPlatformRole := &schema.UserRole{ID: user.PlatformRoleID}
		err = userPlatformRole.Get(r.Context())
		if err != nil {
			return
		}
		if !userPlatformRole.FullAccess {
			nodes = logic.GetFilteredNodesByUserAccess(user, nodes)
		}

	}
	nodes = logic.AddStaticNodestoList(nodes)
	// return all the nodes in JSON/API format
	apiNodes := logic.GetAllNodesAPI(nodes[:])
	for i := range apiNodes {
		apiNodes[i].StaticNode.PrivateKey = ""
	}
	logger.Log(3, r.Header.Get("user"), "fetched all nodes they have access to")
	logic.SortApiNodes(apiNodes[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNodes)
}

// @Summary     Get all nodes status on the network
// @Router      /api/v1/nodes/{network}/status [get]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Success     200 {object} map[string]models.NodeStatus
// @Failure     500 {object} models.ErrorResponse
func getNetworkNodeStatus(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	netID := params["network"]
	// validate network
	err := (&schema.Network{Name: netID}).Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to get network %v", err), "badrequest"))
		return
	}
	var nodes []models.Node
	nodes, err = logic.GetNetworkNodes(netID)
	if err != nil {
		logger.Log(0, "error fetching all nodes info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	nodes = logic.AddStaticNodestoList(nodes)
	// return all the nodes in JSON/API format
	apiNodesStatusMap := logic.GetNodesStatusAPI(nodes[:])
	logger.Log(3, r.Header.Get("user"), "fetched all nodes they have access to")
	logic.ReturnSuccessResponseWithJson(w, r, apiNodesStatusMap, "fetched nodes with metric status")
}

// @Summary     Get an individual node
// @Router      /api/nodes/{network}/{nodeid} [get]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.NodeGet
// @Failure     500 {object} models.ErrorResponse
func getNode(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	nodeid := params["nodeid"]

	node, err := logic.ValidateParams(nodeid, params["network"])
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	host := &schema.Host{
		ID: node.HostID,
	}
	err = host.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching host for node [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf(
				"error fetching wg peers config for host [ %s ]: %v",
				host.ID.String(),
				err,
			),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	hostPeerUpdate, err := logic.GetPeerUpdateForHost(node.Network, host, allNodes, nil, nil)
	if err != nil && !database.IsEmptyRecord(err) {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf(
				"error fetching wg peers config for host [ %s ]: %v",
				host.ID.String(),
				err,
			),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	server := logic.GetServerInfo()
	response := models.NodeGet{
		Node:         node,
		Host:         *host,
		HostPeers:    hostPeerUpdate.Peers,
		Peers:        hostPeerUpdate.NodePeers,
		ServerConfig: server,
		PeerIDs:      hostPeerUpdate.PeerIDs,
	}

	logger.Log(2, r.Header.Get("user"), "fetched node", params["nodeid"])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// == EGRESS ==

// @Summary     Create an egress gateway
// @Router      /api/nodes/{network}/{nodeid}/creategateway [post]
// @Tags        Nodes
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Node ID"
// @Param       body body models.EgressGatewayRequest true "Egress gateway request"
// @Success     200 {object} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func createEgressGateway(w http.ResponseWriter, r *http.Request) {
	var gateway models.EgressGatewayRequest
	var params = mux.Vars(r)
	node, err := logic.ValidateParams(params["nodeid"], params["network"])
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&gateway); err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	gateway.NetID = params["network"]
	gateway.NodeID = params["nodeid"]
	err = logic.ValidateEgressRange(gateway.NetID, gateway.Ranges)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error validating egress range: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	node, err = logic.CreateEgressGateway(gateway)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create egress gateway on node [%s] on network [%s]: %v",
				gateway.NodeID, gateway.NetID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	apiNode := node.ConvertToAPINode()
	logger.Log(
		1,
		r.Header.Get("user"),
		"created egress gateway on node",
		gateway.NodeID,
		"on network",
		gateway.NetID,
	)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go func() {
		if err := mq.NodeUpdate(&node); err != nil {
			slog.Error("error publishing node update to node", "node", node.ID, "error", err)
		}
		mq.PublishPeerUpdate(false)
	}()
}

// @Summary     Delete an egress gateway
// @Router      /api/nodes/{network}/{nodeid}/deletegateway [delete]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func deleteEgressGateway(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.ValidateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	node, err = logic.DeleteEgressGateway(netid, nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete egress gateway on node [%s] on network [%s]: %v",
				nodeid, netid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	apiNode := node.ConvertToAPINode()
	logger.Log(
		1,
		r.Header.Get("user"),
		"deleted egress gateway on node",
		nodeid,
		"on network",
		netid,
	)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go func() {
		if err := mq.NodeUpdate(&node); err != nil {
			slog.Error("error publishing node update to node", "node", node.ID, "error", err)
		}
		mq.PublishPeerUpdate(false)
	}()
}

// @Summary     Update an individual node
// @Router      /api/nodes/{network}/{nodeid} [put]
// @Tags        Nodes
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Node ID"
// @Param       body body models.ApiNode true "Node update data"
// @Success     200 {object} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func updateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	//start here
	nodeid := params["nodeid"]
	currentNode, err := logic.ValidateParams(nodeid, params["network"])
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var newData models.ApiNode
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&newData)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	network := &schema.Network{Name: currentNode.Network}
	err = network.Get(db.WithContext(context.TODO()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if currentNode.Address.IP != nil && currentNode.Address.String() != newData.Address {
		if !orchestrator.GetRepository().NetworkOrchestrator().IsIPv4Unique(r.Context(), network, newData.Address) {
			err = errors.New("ip specified is already allocated:  " + newData.Address)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	if currentNode.Address6.IP != nil && currentNode.Address6.String() != newData.Address6 {
		if !orchestrator.GetRepository().NetworkOrchestrator().IsIPv6Unique(r.Context(), network, newData.Address6) {
			err = errors.New("ip specified is already allocated:  " + newData.Address6)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}

	if !servercfg.IsPro {
		newData.AdditionalRagIps = []string{}
	}
	newNode := newData.ConvertToServerNode(&currentNode)
	if newNode == nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("error converting node"), "badrequest"),
		)
		return
	}
	if currentNode.IsAutoRelay && (!newNode.IsAutoRelay || !newNode.Connected) {
		logic.ResetAutoRelay(newNode)
	}

	if newNode.IsInternetGateway && len(newNode.InetNodeReq.InetNodeClientIDs) > 0 {
		err = logic.ValidateInetGwReq(logic.ConvertModelsNodeToSchemaNode(newNode), newNode.InetNodeReq, newNode.IsInternetGateway && currentNode.IsInternetGateway)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
		newNode.RelayedNodes = append(newNode.RelayedNodes, newNode.InetNodeReq.InetNodeClientIDs...)
		newNode.RelayedNodes = logic.UniqueStrings(newNode.RelayedNodes)
	}

	relayUpdate := logic.RelayUpdates(&currentNode, newNode)
	if relayUpdate && newNode.IsRelay {
		err = logic.ValidateRelay(models.RelayRequest{
			NodeID:       newNode.ID.String(),
			NetID:        newNode.Network,
			RelayedNodes: newNode.RelayedNodes,
		}, true)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	host := &schema.Host{
		ID: newNode.HostID,
	}
	err = host.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get host for node  [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if newNode.IsInternetGateway {
		if host.DNS != "yes" {
			host.DNS = "yes"
			logic.UpsertHost(host)
		}
	}

	err = logic.UpdateNode(&currentNode, newNode)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to update node info [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if relayUpdate {
		logic.UpdateRelayed(&currentNode, newNode)
	}
	if !currentNode.IsInternetGateway && newNode.IsInternetGateway {
		logic.SetInternetGw(newNode, newNode.InetNodeReq)
	}
	if currentNode.IsInternetGateway && newNode.IsInternetGateway {
		// logic.UnsetInternetGw resets newNode.InetNodeReq.
		// So, keeping a copy to pass into logic.SetInternetGw.
		req := newNode.InetNodeReq
		logic.UnsetInternetGw(newNode)
		logic.SetInternetGw(newNode, req)
	}
	if !newNode.IsInternetGateway {
		logic.UnsetInternetGw(newNode)
	}
	if currentNode.AutoAssignGateway && !newNode.AutoAssignGateway {
		// if relayed remove it
		if newNode.IsRelayed {
			relayNode, err := logic.GetNodeByID(newNode.RelayedBy)
			if err == nil {
				logic.RemoveAllFromSlice(relayNode.RelayedNodes, newNode.ID.String())
				logic.UpsertNode(&relayNode)
			}
			newNode.IsRelayed = false
			newNode.RelayedBy = ""
		}
	}
	if (currentNode.IsRelayed) && newNode.AutoAssignGateway {
		// if relayed remove it
		if currentNode.IsRelayed {
			relayNode, err := logic.GetNodeByID(currentNode.RelayedBy)
			if err == nil {
				logic.RemoveAllFromSlice(relayNode.RelayedNodes, currentNode.ID.String())
				logic.UpsertNode(&relayNode)
			}
			newNode.IsRelayed = false
			newNode.RelayedBy = ""
		}
		if len(currentNode.AutoRelayedPeers) > 0 {
			logic.ResetAutoRelayedPeer(&currentNode)
		}
	}
	if !currentNode.AutoAssignGateway && newNode.AutoAssignGateway {
		if len(currentNode.AutoRelayedPeers) > 0 {
			logic.ResetAutoRelayedPeer(&currentNode)
		}
	}
	newNode.PostureChecksViolations,
		newNode.PostureCheckVolationSeverityLevel = logic.CheckPostureViolations(logic.GetPostureCheckDeviceInfoByNode(newNode),
		schema.NetworkID(newNode.Network))
	newNode.LastEvaluatedAt = time.Now().UTC()
	logic.UpsertNode(newNode)

	apiNode := newNode.ConvertToAPINode()
	logger.Log(
		1,
		r.Header.Get("user"),
		"updated node",
		currentNode.ID.String(),
		"on network",
		currentNode.Network,
	)
	logic.LogEvent(&models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   newNode.ID.String(),
			Name: host.Name,
			Type: schema.NodeSub,
		},
		Diff: models.Diff{
			Old: currentNode,
			New: newNode,
		},
		Origin: schema.Dashboard,
	})
	ipChanged := currentNode.Address.String() != newNode.Address.String() ||
		currentNode.Address6.String() != newNode.Address6.String()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go func(relayupdate bool, newNode *models.Node) {
		if err := mq.NodeUpdate(newNode); err != nil {
			slog.Error("error publishing node update to node", "node", newNode.ID, "error", err)
		}
		if ipChanged {
			if err := mq.HostUpdate(&models.HostUpdate{Action: models.RequestPull, Host: *host}); err != nil {
				slog.Error("error sending sync pull to host on ip change", "host", host.ID, "error", err)
			}
		}
		allNodes, err := logic.GetAllNodes()
		if err == nil {
			mq.PublishSingleHostPeerUpdate(host, allNodes, nil, nil, false, nil)
		}
		if servercfg.IsPro && newNode.AutoAssignGateway {
			mq.HostUpdate(&models.HostUpdate{Action: models.CheckAutoAssignGw, Host: *host, Node: *newNode})
		}
		if !newNode.Connected {
			metrics, err := logic.GetMetrics(newNode.ID.String())
			if err == nil {
				for peer, connectivity := range metrics.Connectivity {
					connectivity.Connected = false
					metrics.Connectivity[peer] = connectivity
				}

				_ = logic.UpdateMetrics(newNode.ID.String(), metrics)
			}
			if servercfg.IsPro {
				displacedNodes := logic.DisplaceAutoRelayedNodes(newNode.ID.String())
				for _, dNode := range displacedNodes {
					dHost := &schema.Host{ID: dNode.HostID}
					if err := dHost.Get(db.WithContext(context.TODO())); err != nil {
						slog.Error("disconnect gw: failed to get host for displaced node", "node", dNode.ID, "error", err)
						continue
					}
					mq.HostUpdate(&models.HostUpdate{Action: models.CheckAutoAssignGw, Host: *dHost, Node: dNode})
				}
			}
		}
		mq.PublishPeerUpdate(false)
	}(relayUpdate, newNode)
}

// @Summary     Delete an individual node
// @Router      /api/nodes/{network}/{nodeid} [delete]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Node ID"
// @Param       force query string false "Force delete"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func deleteNode(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)
	var nodeid = params["nodeid"]
	node, err := logic.ValidateParams(nodeid, params["network"])
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	forceDelete := r.URL.Query().Get("force") == "true"
	fromNode := r.Header.Get("requestfrom") == "node"
	purge := forceDelete || fromNode
	if err := logic.DeleteNode(&node, purge); err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("failed to delete node"), "internal"),
		)
		return
	}

	logic.ReturnSuccessResponse(w, r, nodeid+" deleted.")
	logger.Log(1, r.Header.Get("user"), "Deleted node", nodeid, "from network", params["network"])
	go mq.PublishMqUpdatesForDeletedNode(node, !fromNode)
}

// @Summary     Bulk delete nodes
// @Router      /api/v1/nodes/{network}/bulk [delete]
// @Tags        Nodes
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       body body models.BulkDeleteRequest true "List of node IDs to delete"
// @Success     202 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
func bulkDeleteNodes(w http.ResponseWriter, r *http.Request) {
	network := mux.Vars(r)["network"]
	var req models.BulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid request body: %w", err), logic.BadReq))
		return
	}
	if len(req.IDs) == 0 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("no node IDs provided"), logic.BadReq))
		return
	}
	if err := (&schema.Network{Name: network}).Get(r.Context()); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("network %s not found", network), logic.BadReq))
		return
	}
	user := r.Header.Get("user")
	logic.ReturnAcceptedResponse(w, r, fmt.Sprintf("bulk delete of %d node(s) accepted", len(req.IDs)))

	go func() {
		deleted := 0
		var deletedNodes []models.Node
		for _, nodeID := range req.IDs {
			node, err := logic.GetNodeByID(nodeID)
			if err != nil {
				slog.Error("bulk node delete: node not found", "id", nodeID, "error", err)
				continue
			}
			if node.Network != network {
				continue
			}
			if err := logic.DeleteNode(&node, true); err != nil {
				slog.Error("bulk node delete: failed to delete node", "id", nodeID, "error", err)
				continue
			}
			logic.LogEvent(&models.Event{
				Action: schema.Delete,
				Source: models.Subject{
					ID:   user,
					Name: user,
					Type: schema.UserSub,
				},
				TriggeredBy: user,
				Target: models.Subject{
					ID:   node.ID.String(),
					Name: node.ID.String(),
					Type: schema.NodeSub,
				},
				NetworkID: schema.NetworkID(network),
				Origin:    schema.Dashboard,
				Diff:      models.Diff{Old: node, New: nil},
			})
			logger.Log(1, user, "Deleted node", nodeID, "from network", network)
			deletedNodes = append(deletedNodes, node)
			deleted++
		}
		for _, node := range deletedNodes {
			mq.PublishMqUpdatesForDeletedNode(node, true)
		}
		slog.Info("bulk node delete completed", "deleted", deleted, "total", len(req.IDs))
	}()
}

// @Summary     Bulk update node connected status
// @Router      /api/v1/nodes/{network}/bulk/status [put]
// @Tags        Nodes
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       body body models.BulkNodeStatusUpdate true "Node IDs and desired connected state"
// @Success     202 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
func bulkUpdateNodeStatus(w http.ResponseWriter, r *http.Request) {
	network := mux.Vars(r)["network"]
	if err := (&schema.Network{Name: network}).Get(r.Context()); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("network %s not found", network), logic.BadReq))
		return
	}
	var req models.BulkNodeStatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid request body: %w", err), logic.BadReq))
		return
	}
	if len(req.IDs) == 0 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("no node IDs provided"), logic.BadReq))
		return
	}

	eventAction := schema.Connect
	if !req.Connected {
		eventAction = schema.Disconnect
	}
	user := r.Header.Get("user")
	logic.ReturnAcceptedResponse(w, r, fmt.Sprintf("bulk %s of %d node(s) accepted", eventAction, len(req.IDs)))

	go func() {
		var nodeIDs []interface{}
		// filter out invalid node IDs.
		for _, nodeID := range req.IDs {
			node := &schema.Node{
				ID: nodeID,
			}
			exists, err := node.Exists(db.WithContext(context.TODO()))
			if err == nil && exists {
				nodeIDs = append(nodeIDs, nodeID)
			}
		}

		if len(nodeIDs) == 0 {
			return
		}

		nodeUpdate := &schema.Node{
			Connected: req.Connected,
		}
		if req.Connected {
			nodeUpdate.LastCheckIn = time.Now().UTC()
			nodeUpdate.Status = schema.OnlineSt
		} else {
			nodeUpdate.Status = schema.Disconnected
		}
		err := nodeUpdate.UpdateConnectedStatus(
			db.WithContext(context.TODO()),
			dbtypes.WithFilter("id", nodeIDs...),
		)
		if err != nil {
			slog.Error("bulk node status: failed to update nodes connected status", "error", err)
			return
		}

		for i := range nodeIDs {
			nodeID := nodeIDs[i].(string)
			if !req.Connected {
				metrics, err := logic.GetMetrics(nodeID)
				if err == nil {
					for peer, connectivity := range metrics.Connectivity {
						connectivity.Connected = false
						metrics.Connectivity[peer] = connectivity
					}
					_ = logic.UpdateMetrics(nodeID, metrics)
				}
			}
			logic.LogEvent(&models.Event{
				Action: eventAction,
				Source: models.Subject{
					ID:   user,
					Name: user,
					Type: schema.UserSub,
				},
				TriggeredBy: user,
				Target: models.Subject{
					ID:   nodeID,
					Name: nodeID,
					Type: schema.NodeSub,
				},
				NetworkID: schema.NetworkID(network),
				Origin:    schema.Dashboard,
			})

			node, err := logic.GetNodeByID(nodeID)
			if err == nil {
				err = mq.NodeUpdate(&node)
				if err != nil {
					slog.Error("failed to publish node update", "id", nodeID, "error", err)
				}
			}
		}

		mq.PublishPeerUpdate(false)
		slog.Info("bulk node status completed", "action", eventAction, "total", len(req.IDs))
	}()
}
