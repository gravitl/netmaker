package mq

import (
	"context"
	"encoding/json"
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/logger"
)

const DynamicSecSubTopic = "$CONTROL/dynamic-security/#"
const DynamicSecPubTopic = "$CONTROL/dynamic-security/v1"

type DynSecActionType string

var (
	CreateClient            DynSecActionType = "CREATE_CLIENT"
	DisableClient           DynSecActionType = "DISABLE_CLIENT"
	EnableClient            DynSecActionType = "ENABLE_CLIENT"
	DeleteClient            DynSecActionType = "DELETE_CLIENT"
	CreateAdminClient       DynSecActionType = "CREATE_ADMIN_CLIENT"
	ModifyClient            DynSecActionType = "MODIFY_CLIENT"
	DISABLE_EXISTING_ADMINS DynSecActionType = "DISABLE_EXISTING_ADMINS"
)

var (
	CreateClientCmd  = "createClient"
	DisableClientCmd = "disableClient"
	DeleteClientCmd  = "deleteClient"
	ModifyClientCmd  = "modifyClient"
)

var (
	mqDynSecAdmin string = "Netmaker-Admin"
	adminPassword string = "Netmaker-Admin"
)

type MqDynSecGroup struct {
	Groupname string `json:"groupname"`
	Priority  int    `json:"priority"`
}

type MqDynSecRole struct {
	Rolename string `json:"rolename"`
	Priority int    `json:"priority"`
}

type Acl struct {
	AclType  string `json:"acl_type"`
	Topic    string `json:"topic"`
	Priority int    `json:"priority"`
	Allow    bool   `json:"allow"`
}

type MqDynSecCmd struct {
	Command         string          `json:"command"`
	Username        string          `json:"username"`
	Password        string          `json:"password"`
	RoleName        string          `json:"rolename,omitempty"`
	Acls            []Acl           `json:"acls,omitempty"`
	Clientid        string          `json:"clientid"`
	Textname        string          `json:"textname"`
	Textdescription string          `json:"textdescription"`
	Groups          []MqDynSecGroup `json:"groups"`
	Roles           []MqDynSecRole  `json:"roles"`
}

type DynSecAction struct {
	ActionType DynSecActionType
	Payload    MqDynsecPayload
}

type MqDynsecPayload struct {
	Commands []MqDynSecCmd `json:"commands"`
}

var DynSecChan = make(chan DynSecAction, 100)

func DynamicSecManager(ctx context.Context) {
	defer close(DynSecChan)
	for {
		select {
		case <-ctx.Done():
			return
		case dynSecAction := <-DynSecChan:
			d, err := json.Marshal(dynSecAction.Payload)
			if err != nil {
				continue
			}
			if token := mqclient.Publish(DynamicSecPubTopic, 2, false, d); token.Error() != nil {
				logger.Log(0, fmt.Sprintf("failed to perform action [%s]: %v",
					dynSecAction.ActionType, token.Error()))
			}
		}

	}
}

func watchDynSecTopic(client mqtt.Client, msg mqtt.Message) {

	logger.Log(1, fmt.Sprintf("----->WatchDynSecTopic Message: %+v", string(msg.Payload())))

}
