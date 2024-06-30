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
	return nil
}

func (e *EmqxCloud) CreateEmqxDefaultAuthenticator() error { return nil } // ignore

func (e *EmqxCloud) CreateEmqxDefaultAuthorizer() error { return nil } // ignore

func (e *EmqxCloud) CreateDefaultAllowRule() error {
	return nil
}

func (e *EmqxCloud) CreateHostACL(hostID, serverName string) error {
	return nil
}

func (e *EmqxCloud) AppendNodeUpdateACL(hostID, nodeNetwork, nodeID, serverName string) error {
	return nil

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
