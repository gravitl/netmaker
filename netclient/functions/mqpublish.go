package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// Checkin  -- go routine that checks for public or local ip changes, publishes changes
//   if there are no updates, simply "pings" the server as a checkin
func Checkin(ctx context.Context, wg sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			ncutils.Log("Checkin cancelled")
			return
			//delay should be configuraable -> use cfg.Node.NetworkSettings.DefaultCheckInInterval ??
		case <-time.After(time.Second * 60):
			// ncutils.Log("Checkin running")
			//read latest config
			networks, err := ncutils.GetSystemNetworks()
			if err != nil {
				return
			}
			for _, network := range networks {
				if network == ncutils.COMMS_NETWORK_NAME {
					continue
				}
				var cfg *config.ClientConfig
				cfg.Network = network
				cfg.ReadConfig()
				if cfg.Node.IsStatic != "yes" {
					extIP, err := ncutils.GetPublicIP()
					if err != nil {
						ncutils.PrintLog("error encountered checking public ip addresses: "+err.Error(), 1)
					}
					if cfg.Node.Endpoint != extIP && extIP != "" {
						ncutils.PrintLog("endpoint has changed from "+cfg.Node.Endpoint+" to "+extIP, 1)
						cfg.Node.Endpoint = extIP
						if err := PublishNodeUpdate(cfg); err != nil {
							ncutils.Log("could not publish endpoint change")
						}
					}
					intIP, err := getPrivateAddr()
					if err != nil {
						ncutils.PrintLog("error encountered checking private ip addresses: "+err.Error(), 1)
					}
					if cfg.Node.LocalAddress != intIP && intIP != "" {
						ncutils.PrintLog("local Address has changed from "+cfg.Node.LocalAddress+" to "+intIP, 1)
						cfg.Node.LocalAddress = intIP
						if err := PublishNodeUpdate(cfg); err != nil {
							ncutils.Log("could not publish local address change")
						}
					}
				} else if cfg.Node.IsLocal == "yes" && cfg.Node.LocalRange != "" {
					localIP, err := ncutils.GetLocalIP(cfg.Node.LocalRange)
					if err != nil {
						ncutils.PrintLog("error encountered checking local ip addresses: "+err.Error(), 1)
					}
					if cfg.Node.Endpoint != localIP && localIP != "" {
						ncutils.PrintLog("endpoint has changed from "+cfg.Node.Endpoint+" to "+localIP, 1)
						cfg.Node.Endpoint = localIP
						if err := PublishNodeUpdate(cfg); err != nil {
							ncutils.Log("could not publish localip change")
						}
					}
				}
				if err := PingServer(cfg); err != nil {
					ncutils.PrintLog("could not ping server "+err.Error(), 0)
				}
				Hello(cfg, network)
				// ncutils.Log("Checkin complete")
			}
		}
	}
}

// PublishNodeUpdates -- saves node and pushes changes to broker
func PublishNodeUpdate(cfg *config.ClientConfig) error {
	if err := config.Write(cfg, cfg.Network); err != nil {
		return err
	}
	data, err := json.Marshal(cfg.Node)
	if err != nil {
		return err
	}
	if err = publish(cfg, fmt.Sprintf("update/%s", cfg.Node.ID), data, 1); err != nil {
		return err
	}
	return nil
}

// Hello -- ping the broker to let server know node is alive and doing fine
func Hello(cfg *config.ClientConfig, network string) {
	if err := publish(cfg, fmt.Sprintf("ping/%s", cfg.Node.ID), []byte(ncutils.Version), 0); err != nil {
		ncutils.Log(fmt.Sprintf("error publishing ping, %v", err))
		ncutils.Log("running pull on " + cfg.Node.Network + " to reconnect")
		_, err := Pull(cfg.Node.Network, true)
		if err != nil {
			ncutils.Log("could not run pull on " + cfg.Node.Network + ", error: " + err.Error())
		}
	}
}

func publish(cfg *config.ClientConfig, dest string, msg []byte, qos byte) error {
	// setup the keys
	trafficPrivKey, err := auth.RetrieveTrafficKey(cfg.Node.Network)
	if err != nil {
		return err
	}

	serverPubKey, err := ncutils.ConvertBytesToKey(cfg.Node.TrafficKeys.Server)
	if err != nil {
		return err
	}

	client := setupMQTT(true)
	defer client.Disconnect(250)
	encrypted, err := ncutils.Chunk(msg, serverPubKey, trafficPrivKey)
	if err != nil {
		return err
	}

	if token := client.Publish(dest, qos, false, encrypted); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}
