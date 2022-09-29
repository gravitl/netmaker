package mq

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/pbkdf2"
)

var (
	dynamicSecurityFile = "dynamic-security.json"
	dynConfig           = dynJSON{
		Clients: []client{
			{
				Username:   "Netmaker-Admin",
				TextName:   "netmaker admin user",
				Password:   "",
				Salt:       "",
				Iterations: 0,
				Roles: []clientRole{
					{
						Rolename: "admin",
					},
				},
			},
			{
				Username:   "Netmaker-Server",
				TextName:   "netmaker server user",
				Password:   "",
				Salt:       "",
				Iterations: 0,
				Roles: []clientRole{
					{
						Rolename: "server",
					},
				},
			},
			{
				Username:   "netmaker-exporter",
				TextName:   "netmaker metrics exporter",
				Password:   "yl7HZglF4CvCxgjPLLIYc73LRtjEwp2/SAEQXeW5Ta1Dl4RoLN5/gjqiv8xmue+F9LfRk8KICkNbhSYuEfJ7ww==",
				Salt:       "veLl9eN02i+hKkyT",
				Iterations: 101,
				Roles: []clientRole{
					{
						Rolename: "exporter",
					},
				},
			},
		},
		Roles: []role{
			{
				Rolename: "admin",
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
				Rolename: "server",
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
			{
				Rolename: "exporter",
				Acls: []Acl{
					{
						AclType: "publishClientReceive",
						Topic:   "metrics_exporter",
						Allow:   true,
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
)

const DynamicSecSubTopic = "$CONTROL/dynamic-security/#"
const DynamicSecPubTopic = "$CONTROL/dynamic-security/v1"

var mqAdminClient mqtt.Client

var (
	CreateClientCmd  = "createClient"
	DisableClientCmd = "disableClient"
	DeleteClientCmd  = "deleteClient"
	ModifyClientCmd  = "modifyClient"
)

var (
	CreateRoleCmd = "createRole"
	DeleteRoleCmd = "deleteRole"
)

type dynJSON struct {
	Clients    []client         `json:"clients"`
	Roles      []role           `json:"roles"`
	DefaultAcl defaultAccessAcl `json:"defaultACLAccess"`
}

var (
	mqAdminUserName          string = "Netmaker-Admin"
	mqNetmakerServerUserName string = "Netmaker-Server"
)

type clientRole struct {
	Rolename string `json:"rolename"`
}
type client struct {
	Username   string       `json:"username"`
	TextName   string       `json:"textName"`
	Password   string       `json:"password"`
	Salt       string       `json:"salt"`
	Iterations int          `json:"iterations"`
	Roles      []clientRole `json:"roles"`
}

type role struct {
	Rolename string `json:"rolename"`
	Acls     []Acl  `json:"acls"`
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
	AclType  string `json:"acltype"`
	Topic    string `json:"topic"`
	Priority int    `json:"priority,omitempty"`
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
	Payload MqDynsecPayload
}

type MqDynsecPayload struct {
	Commands []MqDynSecCmd `json:"commands"`
}

func encodePasswordToPBKDF2(password string, salt string, iterations int, keyLength int) string {
	binaryEncoded := pbkdf2.Key([]byte(password), []byte(salt), iterations, keyLength, sha512.New)
	return base64.StdEncoding.EncodeToString(binaryEncoded)
}

func Configure() error {
	password := servercfg.GetMqAdminPassword()
	if password == "" {
		return errors.New("MQ admin password not provided")
	}
	for i, cI := range dynConfig.Clients {
		if cI.Username == mqAdminUserName || cI.Username == mqNetmakerServerUserName {
			salt := logic.RandomString(12)
			hashed := encodePasswordToPBKDF2(password, salt, 101, 64)
			cI.Password = hashed
			cI.Iterations = 101
			cI.Salt = base64.StdEncoding.EncodeToString([]byte(salt))
			dynConfig.Clients[i] = cI
		}
	}
	data, err := json.MarshalIndent(dynConfig, "", " ")
	if err != nil {
		return err
	}
	path := functions.GetNetmakerPath() + ncutils.GetSeparator() + dynamicSecurityFile
	return os.WriteFile(path, data, 0755)
}

func PublishEventToDynSecTopic(event DynSecAction) error {

	d, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	var connecterr error
	if token := mqAdminClient.Publish(DynamicSecPubTopic, 2, false, d); !token.WaitTimeout(MQ_TIMEOUT*time.Second) || token.Error() != nil {
		if token.Error() == nil {
			connecterr = errors.New("connect timeout")
		} else {
			connecterr = token.Error()
		}
	}
	return connecterr
}

func watchDynSecTopic(client mqtt.Client, msg mqtt.Message) {

	logger.Log(1, fmt.Sprintf("----->WatchDynSecTopic Message: %+v", string(msg.Payload())))

}
