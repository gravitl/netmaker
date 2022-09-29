package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/servercfg"
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
