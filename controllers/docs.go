// Package classification Netmaker
//
// # API Usage
//
// Most actions that can be performed via API can be performed via UI. We recommend managing your networks using the official netmaker-ui project. However, Netmaker can also be run without the UI, and all functions can be achieved via API calls. If your use case requires using Netmaker without the UI or you need to do some troubleshooting/advanced configuration, using the API directly may help.
//
// # Authentication
//
// API calls must be authenticated via a header of the format -H “Authorization: Bearer <YOUR_SECRET_KEY>” There are two methods to obtain YOUR_SECRET_KEY: 1. Using the masterkey. By default, this value is “secret key,” but you should change this on your instance and keep it secure. This value can be set via env var at startup or in a config file (config/environments/< env >.yaml). See the [Netmaker](https://docs.netmaker.org/index.html) documentation for more details. 2. Using a JWT received for a node. This can be retrieved by calling the /api/nodes/<network>/authenticate endpoint, as documented below.
//
//	Schemes: https
//	BasePath: /
//	Version: 0.17.1
//	Host: netmaker.io
//
//	Consumes:
//	- application/json
//
//	Produces:
//	- application/json
//
//	Security:
//	- oauth
//
// swagger:meta
package controller

import (
	serverconfigpkg "github.com/gravitl/netmaker/config"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
)

var _ = useUnused() // "use" the function to prevent "unused function" errors

// swagger:parameters getNodeDNS getCustomDNS getDNS
type dnsPathParams struct {
	// Network
	// in: path
	Network string `json:"network"`
}

// swagger:parameters createDNS
type dnsParams struct {
	// Network
	// in: path
	Network string `json:"network"`

	// DNS Entry
	// in: body
	Body []models.DNSEntry `json:"body"`
}

// Success
// swagger:response dnsResponse
type dnsResponse struct {
	// in: body
	Body []models.DNSEntry `json:"body"`
}

// swagger:parameters deleteDNS
type dnsDeletePathParams struct {
	// Network
	// in: path
	Network string `json:"network"`

	// Domain
	// in: path
	Domain string `json:"domain"`
}

// swagger:response stringJSONResponse
type stringJSONResponse struct {
	// Response
	// in: body
	Response string `json:"response"`
}

// swagger:parameters getAllExtClients
type getAllClientsRequest struct {
	// Networks
	// in:body
	Networks []string `json:"networks"`
}

// swagger:response extClientSliceResponse
type extClientSliceResponse struct {
	// ExtClients
	// in: body
	ExtClients []models.ExtClient `json:"ext_clients"`
}

// swagger:response extClientResponse
type extClientResponse struct {
	// ExtClient
	// in: body
	ExtClient models.ExtClient `json:"ext_client"`
}

// swagger:response successResponse
type successResponse struct {
	// Success Response
	// in: body
	SuccessResponse models.SuccessResponse `json:"success_response"`
}

// swagger:parameters getExtClient getExtClientConf updateExtClient deleteExtClient
type extClientPathParams struct {
	// Client ID
	// in: path
	ClientID string `json:"clientid"`

	// Network
	// in: path
	Network string `json:"network"`
}

// swagger:parameters updateExtClient
type extClientBodyParam struct {
	// ExtClient
	// in: body
	ExtClient models.ExtClient `json:"ext_client"`
}

// swagger:parameters getNetworkExtClients
type extClientNetworkPathParam struct {
	// Network
	// in: path
	Network string `json:"network"`
}

// swagger:parameters createExtClient
type createExtClientPathParams struct {
	// Network
	// in: path
	Network string `json:"network"`

	// Node ID
	// in: path
	NodeID string `json:"node"`

	// Custom ExtClient
	// in: body
	CustomExtClient models.CustomExtClient `json:"custom_ext_client"`
}

// swagger:parameters getNode updateNode deleteNode createRelay deleteRelay createEgressGateway deleteEgressGateway createIngressGateway deleteIngressGateway uncordonNode
type networkNodePathParams struct {
	// Network
	// in: path
	Network string `json:"network"`

	// Node ID
	// in: path
	NodeID string `json:"nodeid"`
}

// swagger:response byteArrayResponse
type byteArrayResponse struct {
	// in: body
	ByteArray []byte `json:"byte_array"`
}

// swagger:parameters getNetworks
type headerNetworks struct {
	// name: networks
	// in: header
	Networks []string `json:"networks"`
}

// swagger:response getNetworksSliceResponse
type getNetworksSliceResponse struct {
	// Networks
	// in: body
	Networks []models.Network `json:"networks"`
}

// swagger:parameters createNetwork updateNetwork
type networkBodyParam struct {
	// Network
	// in: body
	Network models.Network `json:"network"`
}

// swagger:parameters updateNetwork getNetwork updateNetwork updateNetworkNodeLimit deleteNetwork keyUpdate createAccessKey getAccessKeys deleteAccessKey updateNetworkACL getNetworkACL
type networkPathParam struct {
	// Network Name
	// in: path
	NetworkName string `json:"networkname"`
}

// swagger:parameters deleteAccessKey
type networkAccessKeyNamePathParam struct {
	// Access Key Name
	// in: path
	AccessKeyName string `json:"access_key_name"`
}

// swagger:response networkBodyResponse
type networkBodyResponse struct {
	// Network
	// in: body
	Network models.Network `json:"network"`
}

// swagger:parameters createAccessKey
type accessKeyBodyParam struct {
	// Access Key
	// in: body
	AccessKey models.AccessKey `json:"access_key"`
}

// swagger:response accessKeyBodyResponse
type accessKeyBodyResponse struct {
	// Access Key
	// in: body
	AccessKey models.AccessKey `json:"access_key"`
}

// swagger:response accessKeySliceBodyResponse
type accessKeySliceBodyResponse struct {
	// Access Keys
	// in: body
	AccessKey []models.AccessKey `json:"access_key"`
}

// swagger:parameters updateNetworkACL getNetworkACL
type aclContainerBodyParam struct {
	// ACL Container
	// in: body
	ACLContainer acls.ACLContainer `json:"acl_container"`
}

// swagger:response aclContainerResponse
type aclContainerResponse struct {
	// ACL Container
	// in: body
	ACLContainer acls.ACLContainer `json:"acl_container"`
}

// swagger:response nodeSliceResponse
type nodeSliceResponse struct {
	// Nodes
	// in: body
	Nodes []models.Node `json:"nodes"`
}

// swagger:response nodeResponse
type nodeResponse struct {
	// Node
	// in: body
	Node models.Node `json:"node"`
}

// swagger:parameters updateNode deleteNode
type nodeBodyParam struct {
	// Node
	// in: body
	Node models.Node `json:"node"`
}

// swagger:parameters createRelay
type relayRequestBodyParam struct {
	// Relay Request
	// in: body
	RelayRequest models.RelayRequest `json:"relay_request"`
}

// swagger:parameters createEgressGateway
type egressGatewayBodyParam struct {
	// Egress Gateway Request
	// in: body
	EgressGatewayRequest models.EgressGatewayRequest `json:"egress_gateway_request"`
}

// swagger:parameters authenticate
type authParamBodyParam struct {
	// AuthParams
	// in: body
	AuthParams models.AuthParams `json:"auth_params"`
}

// swagger:response serverConfigResponse
type serverConfigResponse struct {
	// Server Config
	// in: body
	ServerConfig serverconfigpkg.ServerConfig `json:"server_config"`
}

// swagger:response nodeGetResponse
type nodeGetResponse struct {
	// Node Get
	// in: body
	NodeGet models.NodeGet `json:"node_get"`
}

// swagger:response nodeLastModifiedResponse
type nodeLastModifiedResponse struct {
	// Node Last Modified
	// in: body
	NodesLastModified int64 `json:"nodes_last_modified"`
}

// swagger:parameters register
type registerRequestBodyParam struct {
	// Register Request
	// in: body
	RegisterRequest config.RegisterRequest `json:"register_request"`
}

// swagger:response registerResponse
type registerResponse struct {
	// Register Response
	// in: body
	RegisterResponse config.RegisterResponse `json:"register_response"`
}

// swagger:response boolResponse
type boolResponse struct {
	// Boolean Response
	// in: body
	BoolResponse bool `json:"bool_response"`
}

// swagger:parameters createAdmin updateUser updateUserNetworks createUser
type userBodyParam struct {
	// User
	// in: body
	User models.User `json:"user"`
}

// swagger:response userBodyResponse
type userBodyResponse struct {
	// User
	// in: body
	User models.User `json:"user"`
}

// swagger:parameters authenticateUser
type userAuthBodyParam struct {
	// User Auth Params
	// in: body
	UserAuthParams models.UserAuthParams `json:"user_auth_params"`
}

// swagger:parameters updateUser updateUserNetworks updateUserAdm createUser deleteUser getUser
type usernamePathParam struct {
	// Username
	// in: path
	Username string `json:"username"`
}

// prevent issues with integration tests for types just used by Swagger docs.
func useUnused() bool {
	_ = dnsPathParams{}
	_ = dnsParams{}
	_ = dnsResponse{}
	_ = dnsDeletePathParams{}
	_ = stringJSONResponse{}
	_ = getAllClientsRequest{}
	_ = extClientSliceResponse{}
	_ = extClientResponse{}
	_ = successResponse{}
	_ = extClientPathParams{}
	_ = extClientBodyParam{}
	_ = extClientNetworkPathParam{}
	_ = createExtClientPathParams{}
	_ = networkNodePathParams{}
	_ = byteArrayResponse{}
	_ = headerNetworks{}
	_ = getNetworksSliceResponse{}
	_ = networkBodyParam{}
	_ = networkPathParam{}
	_ = networkAccessKeyNamePathParam{}
	_ = networkBodyResponse{}
	_ = accessKeyBodyParam{}
	_ = accessKeyBodyResponse{}
	_ = accessKeySliceBodyResponse{}
	_ = aclContainerBodyParam{}
	_ = aclContainerResponse{}
	_ = nodeSliceResponse{}
	_ = nodeResponse{}
	_ = nodeBodyParam{}
	_ = relayRequestBodyParam{}
	_ = egressGatewayBodyParam{}
	_ = authParamBodyParam{}
	_ = serverConfigResponse{}
	_ = nodeGetResponse{}
	_ = nodeLastModifiedResponse{}
	_ = registerRequestBodyParam{}
	_ = registerResponse{}
	_ = boolResponse{}
	_ = userBodyParam{}
	_ = userBodyResponse{}
	_ = userAuthBodyParam{}
	_ = usernamePathParam{}
	return false
}
