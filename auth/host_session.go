package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/orchestrator"
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
			result.Host.PersistentKeepalive = models.DefaultPersistentKeepAlive
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
		var currentNetworks []string
		if result.ALL {
			_networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
			if err == nil && len(_networks) > 0 {
				for i := range _networks {
					currentNetworks = append(currentNetworks, _networks[i].Name)
				}
			}
		} else if len(result.Network) > 0 {
			currentNetworks = append(currentNetworks, result.Network)
		}
		var netsToAdd []string // track the networks not currently owned by host
		hostNets := logic.GetHostNetworks(result.Host.ID.String())
		for _, newNet := range currentNetworks {
			if !logic.StringSliceContains(hostNets, newNet) {
				if len(result.User) > 0 {
					if !isUserAllowed(result.User, newNet) {
						err = fmt.Errorf("unauthorized user %s attempted to register to network %s", result.User, newNet)
						logger.Log(0, err.Error())
						handleHostRegErr(conn, err)
						return
					}
				}
				netsToAdd = append(netsToAdd, newNet)
			}
		}
		server := logic.GetServerInfo()
		server.TrafficKey = key
		host := result.Host
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
		go CheckNetRegAndHostUpdate(models.EnrollmentKey{Networks: netsToAdd}, &host, result.User)
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
func CheckNetRegAndHostUpdate(key models.EnrollmentKey, host *schema.Host, username string) {
	// publish host update through MQ
	featureFlags := logic.GetFeatureFlags()
	keyTags := make(map[models.TagID]struct{})
	if len(key.Groups) > 0 {
		for _, tagI := range key.Groups {
			keyTags[tagI] = struct{}{}
		}
	}
	for _, netID := range key.Networks {
		network := &schema.Network{Name: netID}
		if err := network.Get(db.WithContext(context.TODO())); err == nil {
			if logic.DoesHostExistInTheNetworkAlready(host, network) {
				continue
			}

			violations, _ := logic.CheckPostureViolations(
				models.PostureCheckDeviceInfo{
					ClientLocation: host.Location,
					ClientVersion:  host.Version,
					OS:             host.OS,
					OSFamily:       host.OSFamily,
					OSVersion:      host.OSVersion,
					KernelVersion:  host.KernelVersion,
					AutoUpdate:     host.AutoUpdate,
					SkipAutoUpdate: true,
					Tags:           keyTags,
				},
				schema.NetworkID(network.Name),
			)
			if len(violations) > 0 {
				logger.Log(0, fmt.Sprintf("skipping joining network %s due to violations", network.Name))
				continue
			}

			if featureFlags.EnableDeviceApproval && !network.AutoJoin {
				if err := (&schema.PendingHost{
					HostID:  host.ID.String(),
					Network: netID,
				}).CheckIfPendingHostExists(db.WithContext(context.TODO())); err == nil {
					continue
				}
				keyB, _ := json.Marshal(key)
				// add host to pending host table
				p := schema.PendingHost{
					ID:            uuid.NewString(),
					HostID:        host.ID.String(),
					Hostname:      host.Name,
					Network:       netID,
					PublicKey:     host.PublicKey.String(),
					OS:            host.OS,
					Location:      host.Location,
					Version:       host.Version,
					EnrollmentKey: keyB,
					RequestedAt:   time.Now().UTC(),
				}
				p.Create(db.WithContext(context.TODO()))
				continue
			}

			_, err := orchestrator.GetRepository().NodeOrchestrator().CreateNode(
				db.WithContext(context.TODO()),
				host,
				network,
				orchestrator.UseKey(&key),
				orchestrator.SkipPublishPeerUpdate(),
			)
			if err != nil {
				logger.Log(0, fmt.Sprintf("failed to add host (%s, %s) to network (%s): %v", host.ID.String(), host.Name, netID, err.Error()))
			} else {
				if len(username) > 0 {
					logic.LogEvent(&models.Event{
						Action: schema.JoinHostToNet,
						Source: models.Subject{
							ID:   username,
							Name: username,
							Type: schema.UserSub,
						},
						TriggeredBy: username,
						Target: models.Subject{
							ID:   host.ID.String(),
							Name: host.Name,
							Type: schema.DeviceSub,
						},
						NetworkID: schema.NetworkID(netID),
						Origin:    schema.Dashboard,
					})
				} else {
					logic.LogEvent(&models.Event{
						Action: schema.JoinHostToNet,
						Source: models.Subject{
							ID:   key.Value,
							Name: key.Tags[0],
							Type: schema.EnrollmentKeySub,
						},
						TriggeredBy: username,
						Target: models.Subject{
							ID:   host.ID.String(),
							Name: host.Name,
							Type: schema.DeviceSub,
						},
						NetworkID: schema.NetworkID(netID),
						Origin:    schema.Dashboard,
					})
				}
			}
		}
	}
	if servercfg.IsMessageQueueBackend() {
		mq.HostUpdate(&models.HostUpdate{
			Action: models.RequestAck,
			Host:   *host,
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
