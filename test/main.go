package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/kr/pretty"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/functions"
)

func main() {
	cfg := &config.ClientConfig{}
	cfg.Network = "short"
	cfg.ReadConfig()
	token, err := functions.Authenticate(cfg)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Println("success", token)
	}
	url := "https://" + cfg.Server.API + "/api/nodes/" + cfg.Network + "/" + cfg.Node.ID

	response, err := api("", http.MethodGet, url, token)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(response.StatusCode, response.Status)
	if response.StatusCode != http.StatusOK {
		resBytes, err := io.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		_, _ = pretty.Println(string(resBytes))
	}
	defer response.Body.Close()
	node := models.Node{}
	if err := json.NewDecoder(response.Body).Decode(&node); err != nil {
		fmt.Println(err)
	}
	pretty.Println(node)
}

func api(data any, method, url, authorization string) (*http.Response, error) {
	var request *http.Request
	var err error
	if data != "" {
		payload, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("error encoding data %w", err)
		}
		request, err = http.NewRequest(method, url, bytes.NewBuffer(payload))
		if err != nil {
			return nil, fmt.Errorf("error creating http request %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
	} else {
		request, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating http request %w", err)
		}
	}
	if authorization != "" {
		request.Header.Set("authorization", "Bearer "+authorization)
	}
	client := http.Client{}
	return client.Do(request)
}
