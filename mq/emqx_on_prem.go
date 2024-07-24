package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gravitl/netmaker/servercfg"
)

type EmqxOnPrem struct {
	URL      string
	UserName string
	Password string
}

const already_exists = "ALREADY_EXISTS"

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

	aclRule struct {
		Topic      string `json:"topic"`
		Permission string `json:"permission"`
		Action     string `json:"action"`
	}

	aclObject struct {
		Rules    []aclRule `json:"rules"`
		Username string    `json:"username,omitempty"`
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

func (e *EmqxOnPrem) GetType() servercfg.Emqxdeploy { return servercfg.EmqxOnPremDeploy }

// CreateEmqxUser - creates an EMQX user
func (e *EmqxOnPrem) CreateEmqxUser(username, password string) error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(&emqxUser{
		UserID:   username,
		Password: password,
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
		if !strings.Contains(string(msg), already_exists) {
			return fmt.Errorf("error creating EMQX user %v", string(msg))
		}
	}
	return nil
}
func (e *EmqxOnPrem) CreateEmqxUserforServer() error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(&emqxUser{
		UserID:   servercfg.GetMqUserName(),
		Password: servercfg.GetMqPassword(),
		Admin:    true,
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
		if !strings.Contains(string(msg), already_exists) {
			return fmt.Errorf("error creating EMQX user %v", string(msg))
		}
	}
	return nil
}

// DeleteEmqxUser - deletes an EMQX user
func (e *EmqxOnPrem) DeleteEmqxUser(username string) error {
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
func (e *EmqxOnPrem) CreateEmqxDefaultAuthenticator() error {
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
		if !strings.ContainsAny(string(msg), "ALREADY_EXISTS") {
			return fmt.Errorf("error creating default EMQX authenticator %v", string(msg))
		}
	}
	return nil
}

// CreateEmqxDefaultAuthorizer - creates a default ACL authorization mechanism based on the built in database
func (e *EmqxOnPrem) CreateEmqxDefaultAuthorizer() error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(&struct {
		Enable bool   `json:"enable"`
		Type   string `json:"type"`
	}{Enable: true, Type: "built_in_database"})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, servercfg.GetEmqxRestEndpoint()+"/api/v5/authorization/sources", bytes.NewReader(payload))
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
	if resp.StatusCode != http.StatusNoContent {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if !strings.ContainsAny(string(msg), "duplicated_authz_source_type") {
			return fmt.Errorf("error creating default EMQX ACL authorization mechanism %v", string(msg))
		}
	}
	return nil
}

// CreateDefaultAllowRule - creates a rule to deny access to all topics for all users by default
// to allow user access to topics use the `mq.CreateUserAccessRule` function
func (e *EmqxOnPrem) CreateDefaultAllowRule() error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(&aclObject{Rules: []aclRule{{Topic: "#", Permission: "allow", Action: "all"}}})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, servercfg.GetEmqxRestEndpoint()+"/api/v5/authorization/sources/built_in_database/all", bytes.NewReader(payload))
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
	if resp.StatusCode != http.StatusNoContent {
		msg, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("error creating default ACL rules %v", string(msg))
	}
	return nil
}
