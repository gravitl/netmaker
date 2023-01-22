package mq

import (
	"errors"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/servercfg"
)

const (
	// constant for admin role
	adminRole = "admin"
	// constant for generic role
	genericRole = "generic"

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
						Rolename: genericRole,
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
				Rolename: genericRole,
				Acls:     fetchGenericAcls(), //TODO fetch generic acls
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
				Rolename: genericRole,
			},
		},
	}
)

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

// genericAcls - fetches generice role related acls
func fetchGenericAcls() []Acl {
	return []Acl{
		{
			AclType:  "publishClientSend",
			Topic:    "#",
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
