package mq

import (
	"encoding/json"
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

func (e *EmqxCloud) CreateEmqxUser(username, pass string, admin bool) error {

	payload := userCreateReq{
		UserName: username,
		Password: pass,
	}
	data, _ := json.Marshal(payload)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, e.URL, strings.NewReader(string(data)))
	if err != nil {
		fmt.Println(err)
		return err
	}
	req.SetBasicAuth(e.AppID, e.AppSecret)
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println(string(body))
	return nil
}

func (e *EmqxCloud) CreateEmqxDefaultAuthenticator() error { return nil }

func (e *EmqxCloud) CreateEmqxDefaultAuthorizer() error { return nil }

func (e *EmqxCloud) CreateDefaultDenyRule() error { return nil }

func (e *EmqxCloud) CreateHostACL(hostID, serverName string) error { return nil }

func (e *EmqxCloud) AppendNodeUpdateACL(hostID, nodeNetwork, nodeID, serverName string) error {
	return nil
}

func (e *EmqxCloud) GetUserACL(username string) (*aclObject, error) { return nil, nil }

func (e *EmqxCloud) DeleteEmqxUser(username string) error { return nil }
