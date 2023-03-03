package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/gravitl/netmaker/servercfg"
)

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

// CreateEmqxDefaultAuthorizer - creates a default ACL authorization mechanism based on the built in database
func CreateEmqxDefaultAuthorizer() error {
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
		return fmt.Errorf("error creating default EMQX ACL authorization mechanism %v", string(msg))
	}
	return nil
}

// GetUserACL - returns ACL rules by username
func GetUserACL(username string) (*aclObject, error) {
	token, err := getEmqxAuthToken()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, servercfg.GetEmqxRestEndpoint()+"/api/v5/authorization/sources/built_in_database/rules/users/"+username, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("content-type", "application/json")
	req.Header.Add("authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching ACL rules %v", string(response))
	}
	body := new(aclObject)
	if err := json.Unmarshal(response, body); err != nil {
		return nil, err
	}
	return body, nil
}

// CreateDefaultDenyRule - creates a rule to deny access to all topics for all users by default
// to allow user access to topics use the `mq.CreateUserAccessRule` function
func CreateDefaultDenyRule() error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(&aclObject{Rules: []aclRule{{Topic: "#", Permission: "deny", Action: "all"}}})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, servercfg.GetEmqxRestEndpoint()+"/api/v5/authorization/sources/built_in_database/rules/all", bytes.NewReader(payload))
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

// CreateHostACL - create host ACL rules
func CreateHostACL(hostID, serverName string) error {
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(&aclObject{
		Username: hostID,
		Rules: []aclRule{
			{
				Topic:      fmt.Sprintf("peers/host/%s/%s", hostID, serverName),
				Permission: "allow",
				Action:     "all",
			},
			{
				Topic:      fmt.Sprintf("host/update/%s/%s", hostID, serverName),
				Permission: "allow",
				Action:     "all",
			},
			{
				Topic:      fmt.Sprintf("dns/all/%s/%s", hostID, serverName),
				Permission: "allow",
				Action:     "all",
			},
			{
				Topic:      fmt.Sprintf("dns/update/%s/%s", hostID, serverName),
				Permission: "allow",
				Action:     "all",
			},
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, servercfg.GetEmqxRestEndpoint()+"/api/v5/authorization/sources/built_in_database/rules/users/"+hostID, bytes.NewReader(payload))
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
		return fmt.Errorf("error adding ACL Rules for user %s Error: %v", hostID, string(msg))
	}
	return nil
}

// a lock required for preventing simultaneous updates to the same ACL object leading to overwriting each other
// might occur when multiple nodes belonging to the same host are created at the same time
var nodeAclMux sync.Mutex

// AppendNodeUpdateACL - adds ACL rule for subscribing to node updates for a node ID
func AppendNodeUpdateACL(hostID, nodeNetwork, nodeID string) error {
	nodeAclMux.Lock()
	defer nodeAclMux.Unlock()
	token, err := getEmqxAuthToken()
	if err != nil {
		return err
	}
	aclObject, err := GetUserACL(hostID)
	if err != nil {
		return err
	}
	aclObject.Rules = append(aclObject.Rules, aclRule{
		Topic:      fmt.Sprintf("node/update/%s/%s", nodeNetwork, nodeID),
		Permission: "allow",
		Action:     "subscribe",
	})
	payload, err := json.Marshal(aclObject)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, servercfg.GetEmqxRestEndpoint()+"/api/v5/authorization/sources/built_in_database/rules/users/"+hostID, bytes.NewReader(payload))
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
		return fmt.Errorf("error adding ACL Rules for user %s Error: %v", hostID, string(msg))
	}
	return nil
}
