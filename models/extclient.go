package models

import (
	"net"
	"sync"
	"time"

	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

// ExtClient - struct for external clients
type ExtClient struct {
	ID                     string   `json:"id,omitempty"`
	ClientID               string   `json:"clientid" bson:"clientid"`
	PrivateKey             string   `json:"privatekey" bson:"privatekey"`
	PublicKey              string   `json:"publickey" bson:"publickey"`
	Network                string   `json:"network" bson:"network"`
	DNS                    string   `json:"dns" bson:"dns"`
	Address                string   `json:"address" bson:"address"`
	Address6               string   `json:"address6" bson:"address6"`
	ExtraAllowedIPs        []string `json:"extraallowedips" bson:"extraallowedips"`
	AllowedIPs             []string `json:"allowed_ips"`
	IngressGatewayID       string   `json:"ingressgatewayid" bson:"ingressgatewayid"`
	IngressGatewayEndpoint string   `json:"ingressgatewayendpoint" bson:"ingressgatewayendpoint"`
	// SelectedInternetEgressID is the internet egress this config file uses for full-tunnel exit (empty = none).
	SelectedInternetEgressID          string              `json:"selected_internet_egress_id" bson:"selected_internet_egress_id"`
	LastModified                      int64               `json:"lastmodified" bson:"lastmodified" swaggertype:"primitive,integer" format:"int64"`
	Enabled                           bool                `json:"enabled" bson:"enabled"`
	OwnerID                           string              `json:"ownerid" bson:"ownerid"`
	DeniedACLs                        map[string]struct{} `json:"deniednodeacls" bson:"acls,omitempty"`
	RemoteAccessClientID              string              `json:"remote_access_client_id"`
	PostUp                            string              `json:"postup" bson:"postup"`
	PostDown                          string              `json:"postdown" bson:"postdown"`
	Tags                              map[TagID]struct{}  `json:"tags"`
	OS                                string              `json:"os"`
	OSFamily                          string              `json:"os_family" yaml:"os_family"`
	OSVersion                         string              `json:"os_version" yaml:"os_version"`
	KernelVersion                     string              `json:"kernel_version" yaml:"kernel_version"`
	ClientVersion                     string              `json:"client_version"`
	DeviceID                          string              `json:"device_id"`
	DeviceName                        string              `json:"device_name"`
	PublicEndpoint                    string              `json:"public_endpoint"`
	Country                           string              `json:"country"`
	Location                          string              `json:"location"`
	PostureChecksViolations           []Violation         `json:"posture_check_violations"`
	PostureCheckVolationSeverityLevel schema.Severity     `json:"posture_check_violation_severity_level"`
	LastEvaluatedAt                   time.Time           `json:"last_evaluated_at"`
	JITExpiresAt                      *time.Time          `json:"jit_expires_at,omitempty" bson:"jit_expires_at,omitempty"`
	Status                            schema.NodeStatus   `json:"status" bson:"status"`
	Mutex                             *sync.Mutex         `json:"-"`
}

func (extPeer *ExtClient) AddressIPNet4() net.IPNet {
	return net.IPNet{IP: net.ParseIP(extPeer.Address), Mask: net.CIDRMask(32, 32)}
}

func (extPeer *ExtClient) AddressIPNet6() net.IPNet {
	return net.IPNet{IP: net.ParseIP(extPeer.Address6), Mask: net.CIDRMask(128, 128)}
}

// ExtClientFromV1 converts a schema.Extclient row (the SQL-native storage
// row) into this domain shape. PostureChecksViolations is intentionally left
// unset here - those persist separately in posture_check_violations_v1,
// keyed by the row's ID, not on the row itself.
func ExtClientFromV1(v1 *schema.Extclient) ExtClient {
	return ExtClient{
		ID:                                v1.ID,
		ClientID:                          v1.Name,
		PrivateKey:                        v1.PrivateKey,
		PublicKey:                         v1.PublicKey,
		Network:                           v1.NetworkID,
		DNS:                               v1.DNS,
		Address:                           v1.Address,
		Address6:                          v1.Address6,
		ExtraAllowedIPs:                   []string(v1.ExtraAllowedIPs),
		AllowedIPs:                        []string(v1.AllowedIPs),
		IngressGatewayID:                  v1.IngressGatewayID,
		IngressGatewayEndpoint:            v1.IngressGatewayEndpoint,
		SelectedInternetEgressID:          v1.SelectedInternetEgressID,
		LastModified:                      v1.UpdatedAt.Unix(),
		Enabled:                           v1.Enabled,
		OwnerID:                           v1.OwnerID,
		DeniedACLs:                        v1.DeniedACLs.Data(),
		RemoteAccessClientID:              v1.RemoteAccessClientID,
		PostUp:                            v1.PostUp,
		PostDown:                          v1.PostDown,
		Tags:                              v1.Tags.Data(),
		OS:                                v1.OS,
		OSFamily:                          v1.OSFamily,
		OSVersion:                         v1.OSVersion,
		KernelVersion:                     v1.KernelVersion,
		ClientVersion:                     v1.ClientVersion,
		DeviceID:                          v1.DeviceID,
		DeviceName:                        v1.DeviceName,
		PublicEndpoint:                    v1.PublicEndpoint,
		Country:                           v1.Country,
		Location:                          v1.Location,
		PostureCheckVolationSeverityLevel: v1.PostureCheckSeverity,
		LastEvaluatedAt:                   v1.PostureCheckLastEvaluatedAt,
		JITExpiresAt:                      v1.JITExpiresAt,
		Status:                            v1.Status,
		Mutex:                             &sync.Mutex{},
	}
}

// ToExtClientV1 converts this domain shape into a schema.Extclient row for
// storage. PostureChecksViolations is intentionally not carried over here -
// those persist separately in posture_check_violations_v1.
func (e *ExtClient) ToExtClientV1() *schema.Extclient {
	return &schema.Extclient{
		Name:                     e.ClientID,
		NetworkID:                e.Network,
		PrivateKey:               e.PrivateKey,
		PublicKey:                e.PublicKey,
		DNS:                      e.DNS,
		Address:                  e.Address,
		Address6:                 e.Address6,
		IngressGatewayID:         e.IngressGatewayID,
		IngressGatewayEndpoint:   e.IngressGatewayEndpoint,
		AllowedIPs:               datatypes.JSONSlice[string](e.AllowedIPs),
		ExtraAllowedIPs:          datatypes.JSONSlice[string](e.ExtraAllowedIPs),
		PostUp:                   e.PostUp,
		PostDown:                 e.PostDown,
		SelectedInternetEgressID: e.SelectedInternetEgressID,
		Enabled:                  e.Enabled,
		OwnerID:                  e.OwnerID,

		PostureCheckSeverity:        e.PostureCheckVolationSeverityLevel,
		PostureCheckLastEvaluatedAt: e.LastEvaluatedAt,

		DeniedACLs:           datatypes.NewJSONType(e.DeniedACLs),
		RemoteAccessClientID: e.RemoteAccessClientID,
		Tags:                 datatypes.NewJSONType(e.Tags),
		OS:                   e.OS,
		OSFamily:             e.OSFamily,
		OSVersion:            e.OSVersion,
		KernelVersion:        e.KernelVersion,
		ClientVersion:        e.ClientVersion,
		DeviceID:             e.DeviceID,
		DeviceName:           e.DeviceName,
		PublicEndpoint:       e.PublicEndpoint,
		Country:              e.Country,
		Location:             e.Location,
		JITExpiresAt:         e.JITExpiresAt,
		Status:               e.Status,
	}
}

// CustomExtClient - struct for CustomExtClient params
type CustomExtClient struct {
	ClientID                   string              `json:"clientid,omitempty"`
	PublicKey                  string              `json:"publickey,omitempty"`
	DNS                        string              `json:"dns,omitempty"`
	ExtraAllowedIPs            []string            `json:"extraallowedips,omitempty"`
	Enabled                    bool                `json:"enabled,omitempty"`
	DeniedACLs                 map[string]struct{} `json:"deniednodeacls" bson:"acls,omitempty"`
	RemoteAccessClientID       string              `json:"remote_access_client_id"` // unique ID (MAC address) of RAC machine
	PostUp                     string              `json:"postup" bson:"postup" validate:"max=1024"`
	PostDown                   string              `json:"postdown" bson:"postdown" validate:"max=1024"`
	Tags                       map[TagID]struct{}  `json:"tags"`
	DeviceID                   string              `json:"device_id"`
	DeviceName                 string              `json:"device_name"`
	IsAlreadyConnectedToInetGw bool                `json:"is_already_connected_to_inet_gw"`
	PublicEndpoint             string              `json:"public_endpoint"`
	OS                         string              `json:"os"`
	OSFamily                   string              `json:"os_family" yaml:"os_family"`
	OSVersion                  string              `json:"os_version"                      yaml:"os_version"`
	KernelVersion              string              `json:"kernel_version" yaml:"kernel_version"`
	ClientVersion              string              `json:"client_version"`
	Country                    string              `json:"country"`
	Location                   string              `json:"location"` //format: lat,long
	UseInternetEgress          *bool               `json:"use_internet_egress,omitempty"`
	SelectedInternetEgressID   string              `json:"selected_internet_egress_id,omitempty"`
}

func ConvertToStaticNode(ext ExtClient) Node {
	if ext.Tags == nil {
		ext.Tags = make(map[TagID]struct{})
	}
	return Node{
		CommonNode: CommonNode{
			Network:  ext.Network,
			Address:  ext.AddressIPNet4(),
			Address6: ext.AddressIPNet6(),
		},
		Tags:                               ext.Tags,
		IsStatic:                           true,
		StaticNode:                         ext,
		IsUserNode:                         ext.RemoteAccessClientID != "" || ext.DeviceID != "",
		Mutex:                              ext.Mutex,
		CountryCode:                        ext.Country,
		Location:                           ext.Location,
		PostureChecksViolations:            ext.PostureChecksViolations,
		PostureCheckViolationSeverityLevel: ext.PostureCheckVolationSeverityLevel,
		LastEvaluatedAt:                    ext.LastEvaluatedAt,
	}
}
