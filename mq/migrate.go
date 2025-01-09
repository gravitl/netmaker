package mq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

func setupmqtt_old() (mqtt.Client, error) {

	opts := mqtt.NewClientOptions()
	opts.AddBroker(os.Getenv("OLD_BROKER_ENDPOINT"))
	id := logic.RandomString(23)
	opts.ClientID = id
	opts.SetUsername(os.Getenv("OLD_MQ_USERNAME"))
	opts.SetPassword(os.Getenv("OLD_MQ_PASSWORD"))
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(time.Second << 2)
	opts.SetKeepAlive(time.Minute)
	opts.SetWriteTimeout(time.Minute)
	mqclient := mqtt.NewClient(opts)

	var connecterr error
	if token := mqclient.Connect(); !token.WaitTimeout(30*time.Second) || token.Error() != nil {
		if token.Error() == nil {
			connecterr = errors.New("connect timeout")
		} else {
			connecterr = token.Error()
		}
		slog.Error("unable to connect to broker", "server", os.Getenv("OLD_BROKER_ENDPOINT"), "error", connecterr)
	}
	return mqclient, nil
}

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
	defer resp.Body.Close()
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

func SendPullSYN() error {
	mqclient, err := setupmqtt_old()
	if err != nil {
		return err
	}
	hosts, err := logic.GetAllHosts()
	if err != nil {
		return err
	}
	for _, host := range hosts {
		host := host
		hostUpdate := models.HostUpdate{
			Action: models.RequestPull,
			Host:   host,
		}
		msg, _ := json.Marshal(hostUpdate)
		var encrypted []byte
		var encryptErr error
		vlt, err := logic.VersionLessThan(host.Version, "v0.30.0")
		if err != nil {
			slog.Warn("error checking version less than", "warn", err)
			continue
		}
		if vlt {
			encrypted, encryptErr = encryptMsg(&host, msg)
			if encryptErr != nil {
				slog.Warn("error encrypt with encryptMsg", "warn", encryptErr)
				continue
			}
		} else {
			zipped, err := compressPayload(msg)
			if err != nil {
				slog.Warn("error compressing message", "warn", err)
				continue
			}
			encrypted, encryptErr = encryptAESGCM(host.TrafficKeyPublic[0:32], zipped)
			if encryptErr != nil {
				slog.Warn("error encrypt with encryptMsg", "warn", encryptErr)
				continue
			}
		}

		logger.Log(0, "sending pull syn to", host.Name)
		mqclient.Publish(fmt.Sprintf("host/update/%s/%s", hostUpdate.Host.ID.String(), servercfg.GetServer()), 0, true, encrypted)
	}
	return nil
}

func KickOutClients() error {
	authToken, err := getEmqxAuthTokenOld()
	if err != nil {
		return err
	}
	hosts, err := logic.GetAllHosts()
	if err != nil {
		slog.Error("failed to migrate emqx: ", "error", err)
		return err
	}

	for _, host := range hosts {
		url := fmt.Sprintf("%s/api/v5/clients/%s", os.Getenv("OLD_EMQX_REST_ENDPOINT"), host.ID.String())
		client := &http.Client{}
		req, err := http.NewRequest(http.MethodDelete, url, nil)
		if err != nil {
			slog.Error("failed to kick out client:", "client", host.ID.String(), "error", err)
			continue
		}
		req.Header.Add("Authorization", "Bearer "+authToken)
		res, err := client.Do(req)
		if err != nil {
			slog.Error("failed to kick out client:", "client", host.ID.String(), "req-error", err)
			continue
		}
		if res.StatusCode != http.StatusNoContent {
			slog.Error("failed to kick out client:", "client", host.ID.String(), "status-code", res.StatusCode)
		}
		res.Body.Close()
	}
	return nil
}
