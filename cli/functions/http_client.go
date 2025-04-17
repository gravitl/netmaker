package functions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/cli/config"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
)

const (
	ambBaseUrl        = "https://api.accounts.netmaker.io"
	TenantUrlTemplate = "https://api-%s.app.prod.netmaker.io"
	ambOauthWssUrl    = "wss://api.accounts.netmaker.io/api/v1/auth/sso"
)

func ssoLogin(endpoint string) string {
	var (
		authToken string
		interrupt = make(chan os.Signal, 1)
		url, _    = url.Parse(endpoint)
		socketURL = fmt.Sprintf("wss://%s/api/oauth/headless", url.Host)
	)
	signal.Notify(interrupt, os.Interrupt)
	conn, _, err := websocket.DefaultDialer.Dial(socketURL, nil)
	if err != nil {
		log.Fatal("error connecting to endpoint ", socketURL, err.Error())
	}
	defer conn.Close()
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("error reading from server: ", err.Error())
	}
	fmt.Printf("Please visit:\n %s \n to authenticate\n", string(msg))
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
			if strings.Contains(string(msg), "JWT: ") {
				authToken = strings.TrimPrefix(string(msg), "JWT: ")
			} else {
				logger.Log(0, "Message from server:", string(msg))
				return
			}
		}
	}()
	for {
		select {
		case <-done:
			return authToken
		case <-interrupt:
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				logger.Log(0, "write close:", err.Error())
			}
			return authToken
		}
	}
}

func getAuthToken(ctx config.Context, force bool) string {
	authToken := os.Getenv("NMCTL_ACCESS_TOKEN")
	if authToken != "" {
		return authToken
	}
	if !force && ctx.AuthToken != "" {
		return ctx.AuthToken
	}
	if !ctx.Saas {
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
		defer res.Body.Close()
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

	if !ctx.SSO {
		sToken, _, err := basicAuthSaasSignin(ctx.Username, ctx.Password)
		if err != nil {
			log.Fatal(err)
		}
		authToken, _, err := tenantLogin(ctx, sToken)
		if err != nil {
			log.Fatal(err)
		}
		config.SetAuthToken(authToken)
		return authToken
	}

	accessToken, err := loginSaaSOauth(&models.SsoLoginReqDto{OauthProvider: "oidc"}, ctx.TenantId)
	if err != nil {
		log.Fatal(err)
	}
	config.SetAuthToken(accessToken)
	return accessToken
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
		// TODO add a retry limit, drop goto
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
			log.Fatalf("Error unmarshalling JSON: %s %s", err, string(resBodyBytes))
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

func basicAuthSaasSignin(email, password string) (string, http.Header, error) {
	payload := models.SignInReqDto{
		FormFields: []models.FormField{
			{
				Id:    "email",
				Value: email,
			},
			{
				Id:    "password",
				Value: password,
			},
		},
	}

	var res models.SignInResDto

	// Create a new HTTP client with a timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create the request body
	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(payload)

	// Create the request
	req, err := http.NewRequest("POST", ambBaseUrl+"/auth/signin", payloadBuf)
	if err != nil {
		return "", http.Header{}, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("rid", "thirdpartyemailpassword")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", http.Header{}, err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return "", http.Header{}, fmt.Errorf("error authenticating: %s", resp.Status)
	}

	// Copy the response headers
	resHeaders := resp.Header

	// Decode the response body
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return "", http.Header{}, err
	}

	sToken := resHeaders.Get(models.ResHeaderKeyStAccessToken)
	encodedAccessToken := url.QueryEscape(sToken)

	return encodedAccessToken, resHeaders, nil
}

func tenantLogin(ctx config.Context, sToken string) (string, string, error) {
	url := fmt.Sprintf("%s/api/v1/tenant/login?tenant_id=%s", ambBaseUrl, ctx.TenantId)

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, url, nil)

	if err != nil {
		return "", "", err
	}
	req.Header.Add("Cookie", fmt.Sprintf("sAccessToken=%s", sToken))

	res, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", "", err
	}

	data := models.TenantLoginResDto{}
	json.Unmarshal(body, &data)

	return data.Response.AuthToken, fmt.Sprintf(TenantUrlTemplate, ctx.TenantId), nil
}

func loginSaaSOauth(payload *models.SsoLoginReqDto, tenantId string) (string, error) {
	socketUrl := ambOauthWssUrl
	// Dial the netmaker server controller
	conn, _, err := websocket.DefaultDialer.Dial(socketUrl, nil)
	if err != nil {
		slog.Error("error connecting to endpoint ", "url", socketUrl, "err", err)
		return "", err
	}

	defer conn.Close()
	return handleServerSSORegisterConn(payload, conn, tenantId)
}

func handleServerSSORegisterConn(payload *models.SsoLoginReqDto, conn *websocket.Conn, tenantId string) (string, error) {
	reqData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if err := conn.WriteMessage(websocket.TextMessage, reqData); err != nil {
		return "", err
	}
	dataCh := make(chan string)
	defer close(dataCh)
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				if msgType < 0 {
					slog.Info("received close message from server")
					return
				}
				if !strings.Contains(err.Error(), "normal") { // Error reading a message from the server
					slog.Error("error msg", "err", err)
				}
				return
			}
			if msgType == websocket.CloseMessage {
				slog.Info("received close message from server")
				return
			}
			if strings.Contains(string(msg), "auth/sso") {
				fmt.Printf("Please visit:\n %s \nto authenticate\n", string(msg))
			} else {
				var res models.SsoLoginData
				if err := json.Unmarshal(msg, &res); err != nil {
					return
				}
				accessToken, _, err := tenantLoginV2(res.AmbAccessToken, tenantId, res.Username)
				if err != nil {
					slog.Error("error logging in tenant", "err", err)
					dataCh <- ""
					return
				}
				dataCh <- accessToken
				return
			}
		}
	}()

	for {
		select {
		case accessToken := <-dataCh:
			if accessToken == "" {
				slog.Info("error getting access token")
				return "", fmt.Errorf("error getting access token")
			}
			return accessToken, nil
		case <-time.After(30 * time.Second):
			slog.Error("authentiation timed out")
			os.Exit(1)
		case <-interrupt:
			slog.Info("interrupt received, closing connection")
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Fatal(err)
			}
			os.Exit(1)
		}
	}
}

func tenantLoginV2(ambJwt, tenantId, email string) (string, string, error) {
	url := fmt.Sprintf("%s/api/v1/tenant/login/custom", ambBaseUrl)
	payload := models.LoginReqDto{
		Email:    email,
		TenantID: tenantId,
	}
	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(payload)

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, payloadBuf)
	if err != nil {
		slog.Error("error creating request", "err", err)
		return "", "", err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", ambJwt))

	res, err := client.Do(req)
	if err != nil {
		slog.Error("error sending request", "err", err)
		return "", "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("error reading response body", "err", err)
		return "", "", err
	}

	data := models.TenantLoginResDto{}
	json.Unmarshal(body, &data)

	return data.Response.AuthToken, fmt.Sprintf(TenantUrlTemplate, tenantId), nil
}
