package auth

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro/netcache"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/gravitl/netmaker/servercfg"
)

// SessionHandler - called by the HTTP router when user
// is calling netclient with --login-server parameter in order to authenticate
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
	var loginMessage promodels.LoginMsg

	err = json.Unmarshal(message, &loginMessage)
	if err != nil {
		logger.Log(0, "Failed to unmarshall data err=", err.Error())
		return
	}
	logger.Log(1, "SSO node join attempted with info network:", loginMessage.Network, "node identifier:", loginMessage.Mac, "user:", loginMessage.User)

	req := new(netcache.CValue)
	req.Value = string(loginMessage.Mac)
	req.Network = loginMessage.Network
	req.Pass = ""
	req.User = ""
	// Add any extra parameter provided in the configuration to the Authorize Endpoint request??
	stateStr := hex.EncodeToString([]byte(logic.RandomString(node_signin_length)))
	if err := netcache.Set(stateStr, req); err != nil {
		logger.Log(0, "Failed to process sso request -", err.Error())
		return
	}
	// Wait for the user to finish his auth flow...
	// TBD: what should be the timeout here ?
	timeout := make(chan bool, 1)
	answer := make(chan string, 1)
	defer close(answer)
	defer close(timeout)

	if _, err = logic.GetNetwork(loginMessage.Network); err != nil {
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			logger.Log(0, "error during message writing:", err.Error())
		}
		return
	}

	if loginMessage.User != "" { // handle basic auth
		// verify that server supports basic auth, then authorize the request with given credentials
		// check if user is allowed to join via node sso
		// i.e. user is admin or user has network permissions
		if !servercfg.IsBasicAuthEnabled() {
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
		}
		_, err := logic.VerifyAuthRequest(models.UserAuthParams{
			UserName: loginMessage.User,
			Password: loginMessage.Password,
		})
		if err != nil {
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
			return
		}
		user, err := isUserIsAllowed(loginMessage.User, loginMessage.Network, false)
		if err != nil {
			err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "error during message writing:", err.Error())
			}
			return
		}
		accessToken, err := requestAccessKey(loginMessage.Network, 1, user.UserName)
		if err != nil {
			req.Pass = fmt.Sprintf("Error from the netmaker controller %s", err.Error())
		} else {
			req.Pass = fmt.Sprintf("AccessToken: %s", accessToken)
		}
		// Give the user the access token via Pass in the DB
		if err = netcache.Set(stateStr, req); err != nil {
			logger.Log(0, "machine failed to complete join on network,", loginMessage.Network, "-", err.Error())
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
					logger.Log(0, "timeout occurred while waiting for SSO on network", loginMessage.Network)
					timeout <- true
					break
				}
				continue
			} else if cachedReq.Pass != "" {
				logger.Log(0, "node SSO process completed for user", cachedReq.User, "on network", loginMessage.Network)
				answer <- cachedReq.Pass
				break
			}
			time.Sleep(500) // try it 2 times per second to see if auth is completed
		}
	}()

	select {
	case result := <-answer:
		// a read from req.answerCh has occurred
		err = conn.WriteMessage(messageType, []byte(result))
		if err != nil {
			logger.Log(0, "Error during message writing:", err.Error())
		}
	case <-timeout:
		logger.Log(0, "Authentication server time out for a node on network", loginMessage.Network)
		// the read from req.answerCh has timed out
		err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			logger.Log(0, "Error during message writing:", err.Error())
		}
	}
	// The entry is not needed anymore, but we will let the producer to close it to avoid panic cases
	if err = netcache.Del(stateStr); err != nil {
		logger.Log(0, "failed to remove node SSO cache entry", err.Error())
	}
	// Cleanly close the connection by sending a close message and then
	// waiting (with timeout) for the server to close the connection.
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		logger.Log(0, "write close:", err.Error())
		return
	}
}
