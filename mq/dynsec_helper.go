package mq

import (
	"errors"
	"fmt"
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
				Acls:     fetchServerAcls(), //TODO fetch generic acls
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

// fetches host related acls
func fetchHostAcls(hostID string) []Acl {
	return []Acl{
		{
			AclType:  "publishClientReceive",
			Topic:    fmt.Sprintf("peers/host/%s/#", hostID),
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientReceive",
			Topic:    fmt.Sprintf("host/update/%s/#", hostID),
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientSend",
			Topic:    fmt.Sprintf("host/serverupdate/%s", hostID),
			Priority: -1,
			Allow:    true,
		},
	}
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
			AclType:  "publishClientReceive",
			Topic:    fmt.Sprintf("proxy/%s/#", network),
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
			Topic:    "proxy/#",
			Priority: -1,
			Allow:    true,
		},
		{
			AclType:  "publishClientSend",
			Topic:    "peers/host/#",
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
			AclType:  "publishClientSend",
			Topic:    "host/update/#",
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
		{
			AclType:  "publishClientReceive",
			Topic:    "host/serverupdate/#",
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
