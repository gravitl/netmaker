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
				Acls:     fetchAdminAcls(),
			},
			{
				Rolename: serverRole,
				Acls:     fetchServerAcls(),
			},
			{
				Rolename: NodeRole,
				Acls:     fetchNodeAcls(),
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
		Acls:     fetchExporterAcls(),
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

// FetchNetworkAcls - fetches network acls
func FetchNetworkAcls(network string) []Acl {
	return []Acl{
		{
			AclType:  "publishClientReceive",
			Topic:    fmt.Sprintf("update/%s/#", network),
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    fmt.Sprintf("peers/%s/#", network),
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "subscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "unsubscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
	}
}

// serverAcls - fetches server role related acls
func fetchServerAcls() []Acl {
	return []Acl{
		{
			AclType:  "publishClientSend",
			Topic:    "peers/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientSend",
			Topic:    "update/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientSend",
			Topic:    "metrics_exporter",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    "ping/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    "update/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    "signal/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    "metrics/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "subscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "unsubscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
	}
}

// fetchNodeAcls - fetches node related acls
func fetchNodeAcls() []Acl {
	// keeping node acls generic as of now.
	return []Acl{

		{
			AclType:  "publishClientSend",
			Topic:    "signal/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientSend",
			Topic:    "update/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientSend",
			Topic:    "ping/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientSend",
			Topic:    "metrics/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "subscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "unsubscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
	}
}

// fetchExporterAcls - fetch exporter role related acls
func fetchExporterAcls() []Acl {
	return []Acl{
		{
			AclType:  "publishClientReceive",
			Topic:    "metrics_exporter",
			Allow:    true,
			Priority: -1,
		},
		{
			AclType:  "subscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "unsubscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
	}
}

// fetchAdminAcls - fetches admin role related acls
func fetchAdminAcls() []Acl {
	return []Acl{
		{
			AclType:  "publishClientSend",
			Topic:    "$CONTROL/dynamic-security/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    "$CONTROL/dynamic-security/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "subscribePattern",
			Topic:    "$CONTROL/dynamic-security/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    "$SYS/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "subscribePattern",
			Topic:    "$SYS/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "subscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "unsubscribePattern",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientSend",
			Topic:    "#",
			Priority: -1,
			Allow:    true,
		},
	}
}
