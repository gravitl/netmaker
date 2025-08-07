package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/hostactions"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
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
	defer netcache.Del(stateStr)
	// Wait for the user to finish his auth flow...
	timeout := make(chan bool, 2)
	answer := make(chan netcache.CValue, 1)
	defer close(answer)
	defer close(timeout)
	if len(registerMessage.User) > 0 { // handle basic auth
		logger.Log(0, "user registration attempted with host:", registerMessage.RegisterHost.Name, "user:", registerMessage.User)

		if !logic.IsBasicAuthEnabled() {
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
		}
		_, err := logic.VerifyAuthRequest(models.UserAuthParams{
			UserName: registerMessage.User,
			Password: registerMessage.Password,
		}, logic.NetclientApp)
		if err != nil {
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
			return
		}
		req.Pass = req.Host.ID.String()
		// user, err := logic.GetUser(req.User)
		// if err != nil {
		// 	logger.Log(0, "failed to get user", req.User, "from database")
		// 	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		// 	if err != nil {
		// 		logger.Log(0, "error during message writing:", err.Error())
		// 	}
		// 	return
		// }
		// if !user.IsAdmin && !user.IsSuperAdmin {
		// 	logger.Log(0, "user", req.User, "is neither an admin or superadmin. denying registeration")
		// 	conn.WriteMessage(messageType, []byte("cannot register with a non-admin or non-superadmin"))
		// 	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		// 	if err != nil {
		// 		logger.Log(0, "error during message writing:", err.Error())
		// 	}
		// 	return
		// }

		if err = netcache.Set(stateStr, req); err != nil { // give the user's host access in the DB
			logger.Log(0, "machine failed to complete join on network,", registerMessage.Network, "-", err.Error())
			return
		}
	} else { // handle SSO / OAuth
		if !logic.IsOAuthConfigured() {
			err = conn.WriteMessage(messageType, []byte("Oauth not configured"))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
			return
		}
		logger.Log(0, "user registration attempted with host:", registerMessage.RegisterHost.Name, "via SSO")
		redirectUrl := fmt.Sprintf("https://%s/api/oauth/register/%s", servercfg.GetAPIConnString(), stateStr)
		err = conn.WriteMessage(messageType, []byte(redirectUrl))
		if err != nil {
			logger.Log(0, "error during message writing:", err.Error())
		}
	}

	go func() {
		for {
			msgType, _, err := conn.ReadMessage()
			if err != nil || msgType == websocket.CloseMessage {
				netcache.Del(stateStr)
				return
			}
		}
	}()

	go func() {
		for {
			cachedReq, err := netcache.Get(stateStr)
			if err != nil {
				logger.Log(0, "oauth state has been deleted ", err.Error())
				timeout <- true
				break

			} else if len(cachedReq.User) > 0 {
				logger.Log(0, "host SSO process completed for user", cachedReq.User)
				answer <- *cachedReq
				break
			}
			time.Sleep(time.Second)
		}
	}()

	select {
	case result := <-answer: // a read from req.answerCh has occurred
		// add the host, if not exists, handle like enrollment registration
		if !logic.HostExists(&result.Host) { // check if host already exists, add if not
			if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
				if err := mq.GetEmqxHandler().CreateEmqxUser(result.Host.ID.String(), result.Host.HostPass); err != nil {
					logger.Log(0, "failed to create host credentials for EMQX: ", err.Error())
					return
				}
			}
			_ = logic.CheckHostPorts(&result.Host)
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
					_, err := isUserIsAllowed(result.User, newNet)
					if err != nil {
						logger.Log(0, "unauthorized user", result.User, "attempted to register to network", newNet)
						handleHostRegErr(conn, err)
						return
					}
				}
				netsToAdd = append(netsToAdd, newNet)
			}
		}
		server := logic.GetServerInfo()
		server.TrafficKey = key
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
		go CheckNetRegAndHostUpdate(models.EnrollmentKey{Networks: netsToAdd}, &result.Host, "")
	case <-timeout: // the read from req.answerCh has timed out
		logger.Log(0, "timeout signal recv,exiting oauth socket conn")
		break
	}
	// Cleanly close the connection by sending a close message and then
	// waiting (with timeout) for the server to close the connection.
	if err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		logger.Log(0, "write close:", err.Error())
		return
	}
}

// CheckNetRegAndHostUpdate - run through networks and send a host update
func CheckNetRegAndHostUpdate(key models.EnrollmentKey, h *models.Host, username string) {
	// publish host update through MQ
	for _, netID := range key.Networks {
		if network, err := logic.GetNetwork(netID); err == nil {
			if network.AutoJoin == "false" {
				if logic.DoesHostExistinTheNetworkAlready(h, models.NetworkID(netID)) {
					continue
				}
				if err := (&schema.PendingHost{
					HostID:  h.ID.String(),
					Network: netID,
				}).CheckIfPendingHostExists(db.WithContext(context.TODO())); err == nil {
					continue
				}
				keyB, _ := json.Marshal(key)
				// add host to pending host table
				p := schema.PendingHost{
					ID:            uuid.NewString(),
					HostID:        h.ID.String(),
					Hostname:      h.Name,
					Network:       netID,
					PublicKey:     h.PublicKey.String(),
					OS:            h.OS,
					Location:      h.Location,
					Version:       h.Version,
					EnrollmentKey: keyB,
					RequestedAt:   time.Now().UTC(),
				}
				p.Create(db.WithContext(context.TODO()))
				continue
			}

			logic.LogEvent(&models.Event{
				Action: models.JoinHostToNet,
				Source: models.Subject{
					ID:   key.Value,
					Name: key.Tags[0],
					Type: models.EnrollmentKeySub,
				},
				TriggeredBy: username,
				Target: models.Subject{
					ID:   h.ID.String(),
					Name: h.Name,
					Type: models.DeviceSub,
				},
				NetworkID: models.NetworkID(netID),
				Origin:    models.Dashboard,
			})

			newNode, err := logic.UpdateHostNetwork(h, netID, true)
			if err == nil || strings.Contains(err.Error(), "host already part of network") {
				if len(key.Groups) > 0 {
					newNode.Tags = make(map[models.TagID]struct{})
					for _, tagI := range key.Groups {
						newNode.Tags[tagI] = struct{}{}
					}
					logic.UpsertNode(newNode)
				}
				if key.Relay != uuid.Nil && !newNode.IsRelayed {
					// check if relay node exists and acting as relay
					relaynode, err := logic.GetNodeByID(key.Relay.String())
					if err == nil && relaynode.IsGw && relaynode.Network == newNode.Network {
						slog.Error(fmt.Sprintf("adding relayed node %s to relay %s on network %s", newNode.ID.String(), key.Relay.String(), netID))
						newNode.IsRelayed = true
						newNode.RelayedBy = key.Relay.String()
						updatedRelayNode := relaynode
						updatedRelayNode.RelayedNodes = append(updatedRelayNode.RelayedNodes, newNode.ID.String())
						logic.UpdateRelayed(&relaynode, &updatedRelayNode)
						if err := logic.UpsertNode(&updatedRelayNode); err != nil {
							slog.Error("failed to update node", "nodeid", key.Relay.String())
						}
						if err := logic.UpsertNode(newNode); err != nil {
							slog.Error("failed to update node", "nodeid", key.Relay.String())
						}
					} else {
						slog.Error("failed to relay node. maybe specified relay node is actually not a relay? Or the relayed node is not in the same network with relay?", "err", err)
					}
				}
				if err != nil && strings.Contains(err.Error(), "host already part of network") {
					continue
				}
			} else {
				logger.Log(0, "failed to add host to network:", h.ID.String(), h.Name, netID, err.Error())
				continue
			}
			logger.Log(1, "added new node", newNode.ID.String(), "to host", h.Name)
			hostactions.AddAction(models.HostUpdate{
				Action: models.JoinHostToNetwork,
				Host:   *h,
				Node:   *newNode,
			})
			if h.IsDefault {
				// make  host failover
				logic.CreateFailOver(*newNode)
				// make host remote access gateway
				logic.CreateIngressGateway(netID, newNode.ID.String(), models.IngressRequest{})
				logic.CreateRelay(models.RelayRequest{
					NodeID: newNode.ID.String(),
					NetID:  netID,
				})
			}
		}
	}
	if servercfg.IsMessageQueueBackend() {
		mq.HostUpdate(&models.HostUpdate{
			Action: models.RequestAck,
			Host:   *h,
		})
		if err := mq.PublishPeerUpdate(false); err != nil {
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
