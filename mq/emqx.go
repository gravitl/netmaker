package mq

import "github.com/gravitl/netmaker/servercfg"

var emqx Emqx

type Emqx interface {
	GetType() servercfg.Emqxdeploy
	CreateEmqxUser(username, password string) error
	CreateEmqxUserforServer() error
	CreateEmqxDefaultAuthenticator() error
	CreateEmqxDefaultAuthorizer() error
	CreateDefaultDenyRule() error
	CreateHostACL(hostID, serverName string) error
	AppendNodeUpdateACL(hostID, nodeNetwork, nodeID, serverName string) error
	GetUserACL(username string) (*aclObject, error)
	DeleteEmqxUser(username string) error
}

func init() {
	if servercfg.GetBrokerType() != servercfg.EmqxBrokerType {
		return
	}
	if servercfg.GetEmqxDeployType() == servercfg.EmqxCloudDeploy {
		emqx = &EmqxCloud{
			URL:       servercfg.GetEmqxRestEndpoint(),
			AppID:     servercfg.GetEmqxAppID(),
			AppSecret: servercfg.GetEmqxAppSecret(),
		}
	} else {
		emqx = &EmqxOnPrem{
			URL:      servercfg.GetEmqxRestEndpoint(),
			UserName: servercfg.GetMqUserName(),
			Password: servercfg.GetMqPassword(),
		}
	}
}

// GetEmqxHandler - gets emqx handler
func GetEmqxHandler() Emqx {
	return emqx
}
