package auth

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/hostactions"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

// SessionHandler - called by the HTTP router when user
// is calling netclient with join/register -s parameter in order to authenticate
// via SSO mechanism by OAuth2 protocol flow.
// This triggers a session start and it is managed by the flow implemented here and callback
// When this method finishes - the auth flow has finished either OK or by timeout or any other error occured
func SessionHandler(conn *websocket.Conn) {
	defer conn.Close()
	// If reached here we have a session from user to handle...
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		logger.Log(0, "Error during message reading:", err.Error())
		return
	}

	var registerMessage models.RegisterMsg
	if err = json.Unmarshal(message, &registerMessage); err != nil {
		logger.Log(0, "Failed to unmarshall data err=", err.Error())
		return
	}
	if registerMessage.RegisterHost.ID == uuid.Nil {
		logger.Log(0, "invalid host registration attempted")
		return
	}

	req := new(netcache.CValue)
	req.Value = string(registerMessage.RegisterHost.ID.String())
	req.Network = registerMessage.Network
	req.Host = registerMessage.RegisterHost
	req.ALL = registerMessage.JoinAll
	req.Pass = ""
	req.User = registerMessage.User
	if len(req.User) > 0 && len(registerMessage.Password) == 0 {
		logger.Log(0, "invalid host registration attempted")
		return
	}
	// Add any extra parameter provided in the configuration to the Authorize Endpoint request??
	stateStr := logic.RandomString(node_signin_length)
	if err := netcache.Set(stateStr, req); err != nil {
		logger.Log(0, "Failed to process sso request -", err.Error())
		return
	}
	// Wait for the user to finish his auth flow...
	timeout := make(chan bool, 1)
	answer := make(chan netcache.CValue, 1)
	defer close(answer)
	defer close(timeout)

	if len(registerMessage.User) > 0 { // handle basic auth
		logger.Log(0, "user registration attempted with host:", registerMessage.RegisterHost.Name, "user:", registerMessage.User)

		if !servercfg.IsBasicAuthEnabled() {
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
		}
		_, err := logic.VerifyAuthRequest(models.UserAuthParams{
			UserName: registerMessage.User,
			Password: registerMessage.Password,
		})
		if err != nil {
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
			return
		}
		req.Pass = req.Host.ID.String()

		if err = netcache.Set(stateStr, req); err != nil { // give the user's host access in the DB
			logger.Log(0, "machine failed to complete join on network,", registerMessage.Network, "-", err.Error())
			return
		}
	} else { // handle SSO / OAuth
		if auth_provider == nil {
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
			return
		}
		logger.Log(0, "user registration attempted with host:", registerMessage.RegisterHost.Name, "via SSO")
		redirectUrl = fmt.Sprintf("https://%s/api/oauth/register/%s", servercfg.GetAPIConnString(), stateStr)
		err = conn.WriteMessage(messageType, []byte(redirectUrl))
		if err != nil {
			logger.Log(0, "error during message writing:", err.Error())
		}
	}

	go func() {
		for {
			cachedReq, err := netcache.Get(stateStr)
			if err != nil {
				if strings.Contains(err.Error(), "expired") {
					logger.Log(1, "timeout occurred while waiting for SSO registration")
					timeout <- true
					break
				}
				continue
			} else if len(cachedReq.User) > 0 {
				logger.Log(0, "host SSO process completed for user", cachedReq.User)
				answer <- *cachedReq
				break
			}
			time.Sleep(500) // try it 2 times per second to see if auth is completed
		}
	}()

	select {
	case result := <-answer: // a read from req.answerCh has occurred
		// add the host, if not exists, handle like enrollment registration
		hostPass := result.Host.HostPass
		if !logic.HostExists(&result.Host) { // check if host already exists, add if not
			if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
				if err := mq.CreateEmqxUser(result.Host.ID.String(), result.Host.HostPass, false); err != nil {
					logger.Log(0, "failed to create host credentials for EMQX: ", err.Error())
					return
				}
				if err := mq.CreateHostACL(result.Host.ID.String(), servercfg.GetServerInfo().Server); err != nil {
					logger.Log(0, "failed to add host ACL rules to EMQX: ", err.Error())
					return
				}
			}
			logic.CheckHostPorts(&result.Host)
			if err := logic.CreateHost(&result.Host); err != nil {
				handleHostRegErr(conn, err)
				return
			}
		}
		key, keyErr := logic.RetrievePublicTrafficKey()
		if keyErr != nil {
			handleHostRegErr(conn, err)
			return
		}
		currHost, err := logic.GetHost(result.Host.ID.String())
		if err != nil {
			handleHostRegErr(conn, err)
			return
		}
		var currentNetworks = []string{}
		if result.ALL {
			currentNets, err := logic.GetNetworks()
			if err == nil && len(currentNets) > 0 {
				for i := range currentNets {
					currentNetworks = append(currentNetworks, currentNets[i].NetID)
				}
			}
		} else if len(result.Network) > 0 {
			currentNetworks = append(currentNetworks, result.Network)
		}
		var netsToAdd = []string{} // track the networks not currently owned by host
		hostNets := logic.GetHostNetworks(currHost.ID.String())
		for _, newNet := range currentNetworks {
			if !logic.StringSliceContains(hostNets, newNet) {
				if len(result.User) > 0 {
					_, err := isUserIsAllowed(result.User, newNet, false)
					if err != nil {
						logger.Log(0, "unauthorized user", result.User, "attempted to register to network", newNet)
						handleHostRegErr(conn, err)
						return
					}
				}
				netsToAdd = append(netsToAdd, newNet)
			}
		}
		server := servercfg.GetServerInfo()
		server.TrafficKey = key
		if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
			// set MQ username and password for EMQX clients
			server.MQUserName = result.Host.ID.String()
			server.MQPassword = hostPass
		}
		result.Host.HostPass = ""
		response := models.RegisterResponse{
			ServerConf:    server,
			RequestedHost: result.Host,
		}
		reponseData, err := json.Marshal(&response)
		if err != nil {
			handleHostRegErr(conn, err)
			return
		}
		if err = conn.WriteMessage(messageType, reponseData); err != nil {
			logger.Log(0, "error during message writing:", err.Error())
		}
		go CheckNetRegAndHostUpdate(netsToAdd[:], &result.Host, uuid.Nil)
	case <-timeout: // the read from req.answerCh has timed out
		if err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
			logger.Log(0, "error during timeout message writing:", err.Error())
		}
	}
	// The entry is not needed anymore, but we will let the producer to close it to avoid panic cases
	if err = netcache.Del(stateStr); err != nil {
		logger.Log(0, "failed to remove node SSO cache entry", err.Error())
	}
	// Cleanly close the connection by sending a close message and then
	// waiting (with timeout) for the server to close the connection.
	if err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		logger.Log(0, "write close:", err.Error())
		return
	}
}

// CheckNetRegAndHostUpdate - run through networks and send a host update
func CheckNetRegAndHostUpdate(networks []string, h *models.Host, relayNodeId uuid.UUID) {
	// publish host update through MQ
	for i := range networks {
		network := networks[i]
		if ok, _ := logic.NetworkExists(network); ok {
			newNode, err := logic.UpdateHostNetwork(h, network, true)
			if err != nil {
				logger.Log(0, "failed to add host to network:", h.ID.String(), h.Name, network, err.Error())
				continue
			}
			if relayNodeId != uuid.Nil && !newNode.IsRelayed {
				newNode.IsRelayed = true
				newNode.RelayedBy = relayNodeId.String()
				slog.Info(fmt.Sprintf("adding relayed node %s to relay %s on network %s", newNode.ID.String(), relayNodeId.String(), network))
				if err := logic.UpsertNode(newNode); err != nil {
					slog.Error("failed to update node", "nodeid", relayNodeId.String())
				}
			}
			logger.Log(1, "added new node", newNode.ID.String(), "to host", h.Name)
			hostactions.AddAction(models.HostUpdate{
				Action: models.JoinHostToNetwork,
				Host:   *h,
				Node:   *newNode,
			})
		}
	}
	if servercfg.IsMessageQueueBackend() {
		mq.HostUpdate(&models.HostUpdate{
			Action: models.RequestAck,
			Host:   *h,
		})
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(0, "failed to publish peer update during registration -", err.Error())
		}
	}
}

func handleHostRegErr(conn *websocket.Conn, err error) {
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		logger.Log(0, "error during host registration via auth:", err.Error())
	}
}
