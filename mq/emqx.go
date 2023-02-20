package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gravitl/netmaker/servercfg"
)

const emqxBrokerType = "emqx"

type (
	emqxUser struct {
		UserID   string `json:"user_id"`
		Password string `json:"password"`
		Admin    bool   `json:"is_superuser"`
	}

	emqxLogin struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	emqxLoginResponse struct {
		License struct {
			Edition string `json:"edition"`
		} `json:"license"`
		Token   string `json:"token"`
		Version string `json:"version"`
	}
)

func getEmqxAuthToken() (string, error) {
	payload, err := json.Marshal(&emqxLogin{
		Username: servercfg.GetMqUserName(),
		Password: servercfg.GetMqPassword(),
	})
	if err != nil {
		return "", err
	}
	resp, err := http.Post(servercfg.GetEmqxRestEndpoint()+"/api/v5/login", "application/json", bytes.NewReader(payload))
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

// CreateEmqxUser - creates an EMQX user
func CreateEmqxUser(username, password string, admin bool) error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(&emqxUser{
		UserID:   username,
		Password: password,
		Admin:    admin,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, servercfg.GetEmqxRestEndpoint()+"/api/v5/authentication/password_based:built_in_database/users", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/json")
	req.Header.Add("authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("error creating EMQX user %v", string(msg))
	}
	return nil
}

// DeleteEmqxUser - deletes an EMQX user
func DeleteEmqxUser(username string) error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodDelete, servercfg.GetEmqxRestEndpoint()+"/api/v5/authentication/password_based:built_in_database/users/"+username, nil)
	if err != nil {
		return err
	}
	req.Header.Add("authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("error deleting EMQX user %v", string(msg))
	}
	return nil
}

// CreateEmqxDefaultAuthenticator - creates a default authenticator based on password and using EMQX's built in database as storage
func CreateEmqxDefaultAuthenticator() error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(&struct {
		Mechanism  string `json:"mechanism"`
		Backend    string `json:"backend"`
		UserIDType string `json:"user_id_type"`
	}{Mechanism: "password_based", Backend: "built_in_database", UserIDType: "username"})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, servercfg.GetEmqxRestEndpoint()+"/api/v5/authentication", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Add("content-type", "application/json")
	req.Header.Add("authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("error creating default EMQX authenticator %v", string(msg))
	}
	return nil
}
