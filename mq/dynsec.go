package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gravitl/netmaker/logger"
)

const DynamicSecTopic = "$CONTROL/dynamic-security/#"

type DynSecActionType string

var (
	CreateClient            DynSecActionType = "CREATE_CLIENT"
	CreateAdminClient       DynSecActionType = "CREATE_ADMIN_CLIENT"
	DISABLE_EXISTING_ADMINS DynSecActionType = "DISABLE_EXISTING_ADMINS"
)

const mqDynSecAdmin = "Netmaker-Admin"
const defaultAdminPassword = "hello-world"

type MqDynSecGroup struct {
	Groupname string `json:"groupname"`
	Priority  int    `json:"priority"`
}

type MqDynSecRole struct {
	Rolename string `json:"rolename"`
	Priority int    `json:"priority"`
}

type MqDynSecCmd struct {
	Command         string          `json:"command"`
	Username        string          `json:"username"`
	Password        string          `json:"password"`
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

	for {
		select {
		case <-ctx.Done():
			return
		case dynSecAction := <-DynSecChan:
			d, err := json.Marshal(dynSecAction.Payload)
			if err != nil {
				continue
			}
			if token := mqclient.Publish(DynamicSecTopic, 2, false, d); token.Error() != nil {
				logger.Log(0, fmt.Sprintf("failed to perform action [%s]: %v",
					dynSecAction.ActionType, token.Error()))
			}
		}

	}
}
