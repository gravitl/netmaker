package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/servercfg"
)

const (
	// constant for admin role
	adminRole = "admin"
	// constant for server role
	serverRole = "server"
	// constant for exporter role
	exporterRole = "exporter"
	// constant for node role
	NodeRole = "node"
	// HostGenericRole constant for host role
	HostGenericRole = "host"

	// const for dynamic security file
	dynamicSecurityFile = "dynamic-security.json"
)

var (
	// default configuration of dynamic security
	dynConfigInI = dynJSON{
		Clients: []client{
			{
				Username:   mqAdminUserName,
				TextName:   "netmaker admin user",
				Password:   "",
				Salt:       "",
				Iterations: 0,
				Roles: []clientRole{
					{
						Rolename: adminRole,
					},
				},
			},
			{
				Username:   mqNetmakerServerUserName,
				TextName:   "netmaker server user",
				Password:   "",
				Salt:       "",
				Iterations: 0,
				Roles: []clientRole{
					{
						Rolename: serverRole,
					},
				},
			},
			exporterMQClient,
		},
		Roles: []role{
			{
				Rolename: adminRole,
			},
			{
				Rolename: serverRole,
			},
			{
				Rolename: HostGenericRole,
			},
			exporterMQRole,
		},
		DefaultAcl: defaultAccessAcl{
			PublishClientSend:    false,
			PublishClientReceive: true,
			Subscribe:            false,
			Unsubscribe:          true,
		},
	}

	exporterMQClient = client{
		Username:   mqExporterUserName,
		TextName:   "netmaker metrics exporter",
		Password:   "",
		Salt:       "",
		Iterations: 101,
		Roles: []clientRole{
			{
				Rolename: exporterRole,
			},
		},
	}
	exporterMQRole = role{
		Rolename: exporterRole,
	}
)

// DynListCLientsCmdResp - struct for list clients response from MQ
type DynListCLientsCmdResp struct {
	Responses []struct {
		Command string          `json:"command"`
		Error   string          `json:"error"`
		Data    ListClientsData `json:"data"`
	} `json:"responses"`
}

// ListClientsData - struct for list clients data
type ListClientsData struct {
	Clients    []string `json:"clients"`
	TotalCount int      `json:"totalCount"`
}

// GetAdminClient - fetches admin client of the MQ
func GetAdminClient() (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	setMqOptions(mqAdminUserName, servercfg.GetMqAdminPassword(), opts)
	mqclient := mqtt.NewClient(opts)
	var connecterr error
	if token := mqclient.Connect(); !token.WaitTimeout(MQ_TIMEOUT*time.Second) || token.Error() != nil {
		if token.Error() == nil {
			connecterr = errors.New("connect timeout")
		} else {
			connecterr = token.Error()
		}
	}
	return mqclient, connecterr
}

// ListClients -  to list all clients in the MQ
func ListClients(client mqtt.Client) (ListClientsData, error) {
	respChan := make(chan mqtt.Message, 10)
	defer close(respChan)
	command := "listClients"
	resp := ListClientsData{}
	msg := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command: command,
			},
		},
	}
	client.Subscribe("$CONTROL/dynamic-security/v1/response", 2, mqtt.MessageHandler(func(c mqtt.Client, m mqtt.Message) {
		respChan <- m
	}))
	defer client.Unsubscribe()
	d, _ := json.Marshal(msg)
	token := client.Publish("$CONTROL/dynamic-security/v1", 2, true, d)
	if !token.WaitTimeout(30) || token.Error() != nil {
		var err error
		if token.Error() == nil {
			err = errors.New("connection timeout")
		} else {
			err = token.Error()
		}
		return resp, err
	}

	for m := range respChan {
		msg := DynListCLientsCmdResp{}
		json.Unmarshal(m.Payload(), &msg)
		for _, mI := range msg.Responses {
			if mI.Command == command {
				return mI.Data, nil
			}
		}
	}
	return resp, errors.New("resp not found")
}

// DeleteNetworkRole - deletes a network role from DynSec system
func DeleteNetworkRole(network string) error {
	// Deletes the network role from MQ
	event := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command:  DeleteRoleCmd,
				RoleName: network,
			},
		},
	}

	return publishEventToDynSecTopic(event)
}

func deleteHostRole(hostID string) error {
	// Deletes the hostID role from MQ
	event := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command:  DeleteRoleCmd,
				RoleName: getHostRoleName(hostID),
			},
		},
	}

	return publishEventToDynSecTopic(event)
}

// CreateNetworkRole - createss a network role from DynSec system
func CreateNetworkRole(network string) error {
	// Create Role with acls for the network
	event := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command:  CreateRoleCmd,
				RoleName: network,
				Textname: "Network wide role with Acls for nodes",
			},
		},
	}

	return publishEventToDynSecTopic(event)
}

// creates role for the host with ID.
func createHostRole(hostID string) error {
	// Create Role with acls for the host
	event := MqDynsecPayload{
		Commands: []MqDynSecCmd{
			{
				Command:  CreateRoleCmd,
				RoleName: getHostRoleName(hostID),
				Textname: "host role with Acls for hosts",
			},
		},
	}

	return publishEventToDynSecTopic(event)
}

func getHostRoleName(hostID string) string {
	return fmt.Sprintf("host-%s", hostID)
}
