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

// mq client for admin
var mqAdminClient mqtt.Client

const (
	// constant for client command
	CreateClientCmd = "createClient"
	// constant for disable command
	DisableClientCmd = "disableClient"
	// constant for delete client command
	DeleteClientCmd = "deleteClient"
	// constant for modify client command
	ModifyClientCmd = "modifyClient"

	// constant for create role command
	CreateRoleCmd = "createRole"
	// constant for delete role command
	DeleteRoleCmd = "deleteRole"

	// constant for admin user name
	mqAdminUserName = "Netmaker-Admin"
	// constant for server user name
	mqNetmakerServerUserName = "Netmaker-Server"
	// constant for exporter user name
	mqExporterUserName = "Netmaker-Exporter"

	// DynamicSecSubTopic - constant for dynamic security subscription topic
	dynamicSecSubTopic = "$CONTROL/dynamic-security/#"
	// DynamicSecPubTopic - constant for dynamic security subscription topic
	dynamicSecPubTopic = "$CONTROL/dynamic-security/v1"
)

// struct for dynamic security file
type dynJSON struct {
	Clients    []client         `json:"clients"`
	Roles      []role           `json:"roles"`
	DefaultAcl defaultAccessAcl `json:"defaultACLAccess"`
}

// struct for client role
type clientRole struct {
	Rolename string `json:"rolename"`
}

// struct for MQ client
type client struct {
	Username   string       `json:"username"`
	TextName   string       `json:"textName"`
	Password   string       `json:"password"`
	Salt       string       `json:"salt"`
	Iterations int          `json:"iterations"`
	Roles      []clientRole `json:"roles"`
}

// struct for MQ role
type role struct {
	Rolename string `json:"rolename"`
	Acls     []Acl  `json:"acls"`
}

// struct for default acls
type defaultAccessAcl struct {
	PublishClientSend    bool `json:"publishClientSend"`
	PublishClientReceive bool `json:"publishClientReceive"`
	Subscribe            bool `json:"subscribe"`
	Unsubscribe          bool `json:"unsubscribe"`
}

// MqDynSecGroup - struct for MQ client group
type MqDynSecGroup struct {
	Groupname string `json:"groupname"`
	Priority  int    `json:"priority"`
}

// MqDynSecRole - struct for MQ client role
type MqDynSecRole struct {
	Rolename string `json:"rolename"`
	Priority int    `json:"priority"`
}

// Acl - struct for MQ acls
type Acl struct {
	AclType  string `json:"acltype"`
	Topic    string `json:"topic"`
	Priority int    `json:"priority,omitempty"`
	Allow    bool   `json:"allow"`
}

// MqDynSecCmd - struct for MQ dynamic security command
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

// MqDynsecPayload - struct for dynamic security command payload
type MqDynsecPayload struct {
	Commands []MqDynSecCmd `json:"commands"`
}

// encodePasswordToPBKDF2 - encodes the given password with PBKDF2 hashing for MQ
func encodePasswordToPBKDF2(password string, salt string, iterations int, keyLength int) string {
	binaryEncoded := pbkdf2.Key([]byte(password), []byte(salt), iterations, keyLength, sha512.New)
	return base64.StdEncoding.EncodeToString(binaryEncoded)
}

// Configure - configures the dynamic initial configuration for MQ
func Configure() error {

	logger.Log(0, "Configuring MQ...")
	dynConfig := dynConfigInI
	path := functions.GetNetmakerPath() + ncutils.GetSeparator() + dynamicSecurityFile

	password := servercfg.GetMqAdminPassword()
	if password == "" {
		return errors.New("MQ admin password not provided")
	}
	if logic.CheckIfFileExists(path) {
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg dynJSON
			err = json.Unmarshal(data, &cfg)
			if err == nil {
				logger.Log(0, "MQ config exists already, So Updating Existing Config...")
				dynConfig = cfg
			}
		}
	}
	exporter := false
	for i, cI := range dynConfig.Clients {
		if cI.Username == mqAdminUserName || cI.Username == mqNetmakerServerUserName {
			salt := logic.RandomString(12)
			hashed := encodePasswordToPBKDF2(password, salt, 101, 64)
			cI.Password = hashed
			cI.Iterations = 101
			cI.Salt = base64.StdEncoding.EncodeToString([]byte(salt))
			dynConfig.Clients[i] = cI
		} else if servercfg.Is_EE && cI.Username == mqExporterUserName {
			exporter = true
			exporterPassword := servercfg.GetLicenseKey()
			salt := logic.RandomString(12)
			hashed := encodePasswordToPBKDF2(exporterPassword, salt, 101, 64)
			cI.Password = hashed
			cI.Iterations = 101
			cI.Salt = base64.StdEncoding.EncodeToString([]byte(salt))
			dynConfig.Clients[i] = cI
		}
	}
	if servercfg.Is_EE && !exporter {
		exporterPassword := servercfg.GetLicenseKey()
		salt := logic.RandomString(12)
		hashed := encodePasswordToPBKDF2(exporterPassword, salt, 101, 64)
		exporterMQClient.Password = hashed
		exporterMQClient.Iterations = 101
		exporterMQClient.Salt = base64.StdEncoding.EncodeToString([]byte(salt))
		dynConfig.Clients = append(dynConfig.Clients, exporterMQClient)
		dynConfig.Roles = append(dynConfig.Roles, exporterMQRole)
	}
	data, err := json.MarshalIndent(dynConfig, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0755)
}

// PublishEventToDynSecTopic - publishes the message to dynamic security topic
func PublishEventToDynSecTopic(payload MqDynsecPayload) error {

	d, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var connecterr error
	if token := mqAdminClient.Publish(dynamicSecPubTopic, 2, false, d); !token.WaitTimeout(MQ_TIMEOUT*time.Second) || token.Error() != nil {
		if token.Error() == nil {
			connecterr = errors.New("connect timeout")
		} else {
			connecterr = token.Error()
		}
	}
	return connecterr
}

// watchDynSecTopic - message handler for dynamic security responses
func watchDynSecTopic(client mqtt.Client, msg mqtt.Message) {

	logger.Log(1, fmt.Sprintf("----->WatchDynSecTopic Message: %+v", string(msg.Payload())))

}
