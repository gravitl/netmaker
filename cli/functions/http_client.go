package functions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/cli/config"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	nmconfig "github.com/gravitl/netmaker/netclient/config"
)

func ssoLogin(endpoint string) string {
	var (
		accessToken *models.AccessToken
		interrupt   = make(chan os.Signal, 1)
		socketURL   = fmt.Sprintf("wss://%s/api/oauth/headless", endpoint)
	)
	signal.Notify(interrupt, os.Interrupt)
	conn, _, err := websocket.DefaultDialer.Dial(socketURL, nil)
	if err != nil {
		log.Fatal("error connecting to endpoint: ", err.Error())
	}
	defer conn.Close()
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("error reading from server: ", err.Error())
	}
	fmt.Printf("Please visit:\n %s \n to authenticate", string(msg))
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				if msgType < 0 {
					done <- struct{}{}
					return
				}
				if !strings.Contains(err.Error(), "normal") {
					log.Fatal("read error: ", err.Error())
				}
				return
			}
			if msgType == websocket.CloseMessage {
				done <- struct{}{}
				return
			}
			if strings.Contains(string(msg), "AccessToken: ") {
				// Access was granted
				rxToken := strings.TrimPrefix(string(msg), "AccessToken: ")
				if accessToken, err = nmconfig.ParseAccessToken(rxToken); err != nil {
					log.Fatalf("failed to parse received access token %s,err=%s\n", accessToken, err.Error())
				}
			} else {
				logger.Log(0, "Message from server:", string(msg))
				return
			}
		}
	}()
	for {
		select {
		case <-done:
			return accessToken.Key
		case <-interrupt:
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "write close:", err.Error())
			}
			return accessToken.Key
		}
	}
}

func getAuthToken(ctx config.Context, force bool) string {
	if !force && ctx.AuthToken != "" {
		return ctx.AuthToken
	}
	if ctx.SSO {
		authToken := ssoLogin(ctx.Endpoint)
		config.SetAuthToken(authToken)
		return authToken
	}
	authParams := &models.UserAuthParams{UserName: ctx.Username, Password: ctx.Password}
	payload, err := json.Marshal(authParams)
	if err != nil {
		log.Fatal(err)
	}
	res, err := http.Post(ctx.Endpoint+"/api/users/adm/authenticate", "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Fatal(err)
	}
	resBodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("Client could not read response body: %s", err)
	}
	if res.StatusCode != http.StatusOK {
		log.Fatalf("Error Status: %d Response: %s", res.StatusCode, string(resBodyBytes))
	}
	body := new(models.SuccessResponse)
	if err := json.Unmarshal(resBodyBytes, body); err != nil {
		log.Fatalf("Error unmarshalling JSON: %s", err)
	}
	authToken := body.Response.(map[string]any)["AuthToken"].(string)
	config.SetAuthToken(authToken)
	return authToken
}

func request[T any](method, route string, payload any) *T {
	var (
		_, ctx = config.GetCurrentContext()
		req    *http.Request
		err    error
	)
	if payload == nil {
		req, err = http.NewRequest(method, ctx.Endpoint+route, nil)
		if err != nil {
			log.Fatalf("Client could not create request: %s", err)
		}
	} else {
		payloadBytes, jsonErr := json.Marshal(payload)
		if jsonErr != nil {
			log.Fatalf("Error in request JSON marshalling: %s", err)
		}
		req, err = http.NewRequest(method, ctx.Endpoint+route, bytes.NewReader(payloadBytes))
		if err != nil {
			log.Fatalf("Client could not create request: %s", err)
		}
		req.Header.Set("Content-Type", "application/json")
	}
	if ctx.MasterKey != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.MasterKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+getAuthToken(ctx, false))
	}
	retried := false
retry:
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Client error making http request: %s", err)
	}
	// refresh JWT token
	if res.StatusCode == http.StatusUnauthorized && !retried && ctx.MasterKey == "" {
		req.Header.Set("Authorization", "Bearer "+getAuthToken(ctx, true))
		retried = true
		goto retry
	}
	resBodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("Client could not read response body: %s", err)
	}
	if res.StatusCode != http.StatusOK {
		log.Fatalf("Error Status: %d Response: %s", res.StatusCode, string(resBodyBytes))
	}
	body := new(T)
	if len(resBodyBytes) > 0 {
		if err := json.Unmarshal(resBodyBytes, body); err != nil {
			log.Fatalf("Error unmarshalling JSON: %s", err)
		}
	}
	return body
}

func get(route string) string {
	_, ctx := config.GetCurrentContext()
	req, err := http.NewRequest(http.MethodGet, ctx.Endpoint+route, nil)
	if err != nil {
		log.Fatal(err)
	}
	if ctx.MasterKey != "" {
		req.Header.Set("Authorization", "Bearer "+ctx.MasterKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+getAuthToken(ctx, true))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	return string(bodyBytes)
}
