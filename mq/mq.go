package mq

import (
	"context"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
)

// KEEPALIVE_TIMEOUT - time in seconds for timeout
const KEEPALIVE_TIMEOUT = 60 //timeout in seconds
// MQ_DISCONNECT - disconnects MQ
const MQ_DISCONNECT = 250

// MQ_TIMEOUT - timeout for MQ
const MQ_TIMEOUT = 30

var peer_force_send = 0

var mqclient mqtt.Client

// SetupMQTT creates a connection to broker and return client
func SetupMQTT() {
	opts := mqtt.NewClientOptions()
	broker, secure := servercfg.GetMessageQueueEndpoint()
	opts.AddBroker(broker)
	id := ncutils.MakeRandomString(23)
	opts.ClientID = id
	if secure {
		opts.SetTLSConfig(&serverctl.TlsConfig)
	}
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(time.Second << 2)
	opts.SetKeepAlive(time.Minute)
	opts.SetWriteTimeout(time.Minute)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		if token := client.Subscribe("ping/#", 2, mqtt.MessageHandler(Ping)); token.WaitTimeout(MQ_TIMEOUT*time.Second) && token.Error() != nil {
			client.Disconnect(240)
			logger.Log(0, "ping subscription failed")
		}
		if token := client.Subscribe("update/#", 0, mqtt.MessageHandler(UpdateNode)); token.WaitTimeout(MQ_TIMEOUT*time.Second) && token.Error() != nil {
			client.Disconnect(240)
			logger.Log(0, "node update subscription failed")
		}
		if token := client.Subscribe("signal/#", 0, mqtt.MessageHandler(ClientPeerUpdate)); token.WaitTimeout(MQ_TIMEOUT*time.Second) && token.Error() != nil {
			client.Disconnect(240)
			logger.Log(0, "node client subscription failed")
		}

		opts.SetOrderMatters(true)
		opts.SetResumeSubs(true)
	})
	mqclient = mqtt.NewClient(opts)
	tperiod := time.Now().Add(10 * time.Second)
	for {
		if token := mqclient.Connect(); !token.WaitTimeout(MQ_TIMEOUT*time.Second) || token.Error() != nil {
			logger.Log(2, "unable to connect to broker, retrying ...")
			if time.Now().After(tperiod) {
				if token.Error() == nil {
					logger.FatalLog("could not connect to broker, token timeout, exiting ...")
				} else {
					logger.FatalLog("could not connect to broker, exiting ...", token.Error().Error())
				}
			}
		} else {
			break
		}
		time.Sleep(2 * time.Second)
	}
}

// Keepalive -- periodically pings all nodes to let them know server is still alive and doing well
func Keepalive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * KEEPALIVE_TIMEOUT):
			sendPeers()
		}
	}
}
