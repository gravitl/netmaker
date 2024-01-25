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

func (e *EmqxCloud) CreateEmqxUser(username, pass string, admin bool) error {

	payload := userCreateReq{
		UserName: username,
		Password: pass,
	}
	data, _ := json.Marshal(payload)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, e.URL, strings.NewReader(string(data)))
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

func (e *EmqxCloud) CreateEmqxDefaultAuthenticator() error { return nil } // ignore

func (e *EmqxCloud) CreateEmqxDefaultAuthorizer() error { return nil } // ignore

func (e *EmqxCloud) CreateDefaultDenyRule() error {
	return nil
}

func (e *EmqxCloud) CreateHostACL(hostID, serverName string) error {
	acls := []cloudAcl{
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("peers/host/%s/%s", hostID, serverName),
			Access:   "allow",
			Action:   "pubsub",
		},
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("host/update/%s/%s", hostID, serverName),
			Access:   "allow",
			Action:   "pubsub",
		},
		{
			UserName: hostID,
			Topic:    fmt.Sprintf("host/serverupdate/%s/%s", serverName, hostID),
			Access:   "allow",
			Action:   "pubsub",
		},
	}
	payload, err := json.Marshal(acls)
	if err != nil {
		return err
	}
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, e.URL, strings.NewReader(string(payload)))
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

func (e *EmqxCloud) AppendNodeUpdateACL(hostID, nodeNetwork, nodeID, serverName string) error {
	return nil
}

func (e *EmqxCloud) GetUserACL(username string) (*aclObject, error) { return nil, nil }

func (e *EmqxCloud) DeleteEmqxUser(username string) error {

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, e.URL, nil)
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
