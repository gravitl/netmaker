package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"golang.org/x/exp/slog"
)

func getEmqxAuthTokenOld() (string, error) {
	payload, err := json.Marshal(&emqxLogin{
		Username: os.Getenv("OLD_MQ_USERNAME"),
		Password: os.Getenv("OLD_MQ_PASSWORD"),
	})
	if err != nil {
		return "", err
	}
	resp, err := http.Post(os.Getenv("OLD_EMQX_REST_ENDPOINT")+"/api/v5/login", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	msg, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error during EMQX login %v", string(msg))
	}
	var loginResp emqxLoginResponse
	if err := json.Unmarshal(msg, &loginResp); err != nil {
		return "", err
	}
	return loginResp.Token, nil
}

func KickOutClients(clientIDs []string) error {
	authToken, err := getEmqxAuthTokenOld()
	if err != nil {
		return err
	}
	for _, clientID := range clientIDs {
		url := fmt.Sprintf("%s/api/v5/clients/%s", os.Getenv("OLD_EMQX_REST_ENDPOINT"), clientID)
		client := &http.Client{}
		req, err := http.NewRequest(http.MethodDelete, url, nil)
		if err != nil {
			slog.Error("failed to kick out client:", "client", clientID, "error", err)
			continue
		}
		req.Header.Add("Authorization", "Bearer "+authToken)
		res, err := client.Do(req)
		if err != nil {
			slog.Error("failed to kick out client:", "client", clientID, "req-error", err)
			continue
		}
		if res.StatusCode != http.StatusNoContent {
			slog.Error("failed to kick out client:", "client", clientID, "status-code", res.StatusCode)
		}
		res.Body.Close()
	}
	return nil
}
