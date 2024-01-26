package mq

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gravitl/netmaker/servercfg"
)

type EmqxCloud struct {
	URL       string
	AppID     string
	AppSecret string
}

type userCreateReq struct {
	UserName string `json:"username"`
	Password string `json:"password"`
}

type cloudAcl struct {
	UserName string `json:"username"`
	Topic    string `json:"topic"`
	Action   string `json:"action"`
	Access   string `json:"access"`
}

func (e *EmqxCloud) GetType() servercfg.Emqxdeploy { return servercfg.EmqxCloudDeploy }

func (e *EmqxCloud) CreateEmqxUser(username, pass string) error {

	payload := userCreateReq{
		UserName: username,
		Password: pass,
	}
	data, _ := json.Marshal(payload)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/auth_username", e.URL), strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(e.AppID, e.AppSecret)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("request failed " + string(body))
	}
	return nil
}

func (e *EmqxCloud) CreateEmqxUserforServer() error {
	payload := userCreateReq{
		UserName: servercfg.GetMqUserName(),
		Password: servercfg.GetMqPassword(),
	}
	data, _ := json.Marshal(payload)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/auth_username", e.URL), strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.SetBasicAuth(e.AppID, e.AppSecret)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("request failed " + string(body))
	}
	// add acls
	acls := []cloudAcl{
		{
			UserName: servercfg.GetMqUserName(),
			Topic:    fmt.Sprintf("update/%s/#", servercfg.GetServer()),
			Access:   "allow",
			Action:   "sub",
		},
		{
			UserName: servercfg.GetMqUserName(),
			Topic:    fmt.Sprintf("host/serverupdate/%s/#", servercfg.GetServer()),
			Access:   "allow",
			Action:   "sub",
		},
		{
			UserName: servercfg.GetMqUserName(),
			Topic:    fmt.Sprintf("signal/%s/#", servercfg.GetServer()),
			Access:   "allow",
			Action:   "sub",
		},
		{
			UserName: servercfg.GetMqUserName(),
			Topic:    fmt.Sprintf("metrics/%s/#", servercfg.GetServer()),
			Access:   "allow",
			Action:   "sub",
		},
		{
			UserName: servercfg.GetMqUserName(),
			Topic:    "peers/host/#",
			Access:   "allow",
			Action:   "pub",
		},
		{
			UserName: servercfg.GetMqUserName(),
			Topic:    "node/update/#",
			Access:   "allow",
			Action:   "pub",
		},
		{

			UserName: servercfg.GetMqUserName(),
			Topic:    "host/update/#",
			Access:   "allow",
			Action:   "pub",
		},
	}

	return e.createacls(acls)
}

func (e *EmqxCloud) CreateEmqxDefaultAuthenticator() error { return nil } // ignore

func (e *EmqxCloud) CreateEmqxDefaultAuthorizer() error { return nil } // ignore

func (e *EmqxCloud) CreateDefaultDenyRule() error {
	return nil
}

func (e *EmqxCloud) createacls(acls []cloudAcl) error {
	payload, err := json.Marshal(acls)
	if err != nil {
		return err
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/acl", e.URL), strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(e.AppID, e.AppSecret)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("request failed " + string(body))
	}
	return nil
}

func (e *EmqxCloud) CreateHostACL(hostID, serverName string) error {
	acls := []cloudAcl{
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("peers/host/%s/%s", hostID, serverName),
			Access:   "allow",
			Action:   "sub",
		},
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("host/update/%s/%s", hostID, serverName),
			Access:   "allow",
			Action:   "sub",
		},
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("host/serverupdate/%s/%s", serverName, hostID),
			Access:   "allow",
			Action:   "pub",
		},
	}

	return e.createacls(acls)
}

func (e *EmqxCloud) AppendNodeUpdateACL(hostID, nodeNetwork, nodeID, serverName string) error {
	acls := []cloudAcl{
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("node/update/%s/%s", nodeNetwork, nodeID),
			Access:   "allow",
			Action:   "sub",
		},
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("ping/%s/%s", serverName, nodeID),
			Access:   "allow",
			Action:   "pubsub",
		},
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("update/%s/%s", serverName, nodeID),
			Access:   "allow",
			Action:   "pubsub",
		},
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("signal/%s/%s", serverName, nodeID),
			Access:   "allow",
			Action:   "pubsub",
		},
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("metrics/%s/%s", serverName, nodeID),
			Access:   "allow",
			Action:   "pubsub",
		},
	}

	return e.createacls(acls)
}

func (e *EmqxCloud) GetUserACL(username string) (*aclObject, error) { return nil, nil } // ununsed on cloud since it doesn't overwrite acls list

func (e *EmqxCloud) DeleteEmqxUser(username string) error {

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/auth_username/%s", e.URL, username), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(e.AppID, e.AppSecret)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return errors.New("request failed " + string(body))
	}
	return nil
}
