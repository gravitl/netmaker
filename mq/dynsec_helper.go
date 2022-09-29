package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/servercfg"
)

var (
	AdminRole    string = "admin"
	ServerRole   string = "server"
	ExporterRole string = "exporter"
)

var (
	dynamicSecurityFile = "dynamic-security.json"
	dynConfig           = dynJSON{
		Clients: []client{
			{
				Username:   mqAdminUserName,
				TextName:   "netmaker admin user",
				Password:   "",
				Salt:       "",
				Iterations: 0,
				Roles: []clientRole{
					{
						Rolename: AdminRole,
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
						Rolename: ServerRole,
					},
				},
			},
		},
		Roles: []role{
			{
				Rolename: AdminRole,
				Acls: []Acl{
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
				},
			},
			{
				Rolename: ServerRole,
				Acls: []Acl{
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
				},
			},
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
				Rolename: ExporterRole,
			},
		},
	}
	exporterMQRole = role{
		Rolename: ExporterRole,
		Acls: []Acl{
			{
				AclType:  "publishClientReceive",
				Topic:    "metrics_exporter",
				Allow:    true,
				Priority: -1,
			},
		},
	}
)

type DynListCLientsCmdResp struct {
	Responses []struct {
		Command string          `json:"command"`
		Error   string          `json:"error"`
		Data    ListClientsData `json:"data"`
	} `json:"responses"`
}

type ListClientsData struct {
	Clients    []string `json:"clients"`
	TotalCount int      `json:"totalCount"`
}

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

func FetchNetworkAcls(network string) []Acl {
	return []Acl{
		{
			AclType: "publishClientReceive",
			Topic:   fmt.Sprintf("update/%s/#", network),
			Allow:   true,
		},
		{
			AclType: "publishClientReceive",
			Topic:   fmt.Sprintf("peers/%s/#", network),
			Allow:   true,
		},
	}
}

func FetchNodeAcls(nodeID string) []Acl {
	return []Acl{

		{
			AclType: "publishClientSend",
			Topic:   fmt.Sprintf("signal/%s", nodeID),
			Allow:   true,
		},
		{
			AclType: "publishClientSend",
			Topic:   fmt.Sprintf("update/%s", nodeID),
			Allow:   true,
		},
		{
			AclType: "publishClientSend",
			Topic:   fmt.Sprintf("ping/%s", nodeID),
			Allow:   true,
		},
		{
			AclType: "publishClientSend",
			Topic:   fmt.Sprintf("metrics/%s", nodeID),
			Allow:   true,
		},
	}
}
