package mq

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/pbkdf2"
)

const DynamicSecSubTopic = "$CONTROL/dynamic-security/#"
const DynamicSecPubTopic = "$CONTROL/dynamic-security/v1"

type DynSecActionType string

var mqAdminClient mqtt.Client

var (
	CreateClient  DynSecActionType = "CREATE_CLIENT"
	DisableClient DynSecActionType = "DISABLE_CLIENT"
	EnableClient  DynSecActionType = "ENABLE_CLIENT"
	DeleteClient  DynSecActionType = "DELETE_CLIENT"
	ModifyClient  DynSecActionType = "MODIFY_CLIENT"
)

var (
	CreateClientCmd  = "createClient"
	DisableClientCmd = "disableClient"
	DeleteClientCmd  = "deleteClient"
	ModifyClientCmd  = "modifyClient"
)

var (
	mqAdminUserName          string = "Netmaker-Admin"
	mqNetmakerServerUserName string = "Netmaker-Server"
)

type client struct {
	Username   string `json:"username"`
	TextName   string `json:"textName"`
	Password   string `json:"password"`
	Salt       string `json:"salt"`
	Iterations int    `json:"iterations"`
	Roles      []struct {
		Rolename string `json:"rolename"`
	} `json:"roles"`
}

type role struct {
	Rolename string `json:"rolename"`
	Acls     []struct {
		Acltype string `json:"acltype"`
		Topic   string `json:"topic"`
		Allow   bool   `json:"allow"`
	} `json:"acls"`
}

type defaultAccessAcl struct {
	PublishClientSend    bool `json:"publishClientSend"`
	PublishClientReceive bool `json:"publishClientReceive"`
	Subscribe            bool `json:"subscribe"`
	Unsubscribe          bool `json:"unsubscribe"`
}

type dynCnf struct {
	Clients          []client         `json:"clients"`
	Roles            []role           `json:"roles"`
	DefaultACLAccess defaultAccessAcl `json:"defaultACLAccess"`
}

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

func encodePasswordToPBKDF2(password string, salt string, iterations int, keyLength int) string {
	binaryEncoded := pbkdf2.Key([]byte(password), []byte(salt), iterations, keyLength, sha512.New)
	return base64.StdEncoding.EncodeToString(binaryEncoded)
}

func Configure() error {
	file := "/root/dynamic-security.json"
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	c := dynCnf{}
	err = json.Unmarshal(b, &c)
	if err != nil {
		return err
	}
	password := servercfg.GetMqAdminPassword()
	if password == "" {
		return errors.New("MQ admin password not provided")
	}
	for i, cI := range c.Clients {
		if cI.Username == mqAdminUserName || cI.Username == mqNetmakerServerUserName {
			salt := logic.RandomString(12)
			hashed := encodePasswordToPBKDF2(password, salt, 101, 64)
			cI.Password = hashed
			cI.Salt = base64.StdEncoding.EncodeToString([]byte(salt))
			c.Clients[i] = cI
		}
	}
	data, err := json.MarshalIndent(c, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0755)
}

func PublishEventToDynSecTopic(event DynSecAction) error {

	d, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	if token := mqAdminClient.Publish(DynamicSecPubTopic, 2, false, d); token.Error() != nil {
		return err
	}
	return nil
}

func watchDynSecTopic(client mqtt.Client, msg mqtt.Message) {

	logger.Log(1, fmt.Sprintf("----->WatchDynSecTopic Message: %+v", string(msg.Payload())))

}
