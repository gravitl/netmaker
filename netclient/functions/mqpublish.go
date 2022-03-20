package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// Checkin  -- go routine that checks for public or local ip changes, publishes changes
//   if there are no updates, simply "pings" the server as a checkin
func Checkin(ctx context.Context, wg *sync.WaitGroup, currentComms map[string]bool) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			logger.Log(0, "checkin routine closed")
			return
			//delay should be configuraable -> use cfg.Node.NetworkSettings.DefaultCheckInInterval ??
		case <-time.After(time.Second * 60):
			// logger.Log(0, "Checkin running")
			//read latest config
			networks, err := ncutils.GetSystemNetworks()
			if err != nil {
				return
			}
			for commsNet := range currentComms {
				var currCommsCfg config.ClientConfig
				currCommsCfg.Network = commsNet
				currCommsCfg.ReadConfig()
				for _, network := range networks {
					var nodeCfg config.ClientConfig
					nodeCfg.Network = network
					nodeCfg.ReadConfig()
					if nodeCfg.Node.CommID != commsNet {
						continue // skip if not on current comms network
					}
					if nodeCfg.Node.IsStatic != "yes" {
						extIP, err := ncutils.GetPublicIP()
						if err != nil {
							logger.Log(1, "error encountered checking public ip addresses: ", err.Error())
						}
						if nodeCfg.Node.Endpoint != extIP && extIP != "" {
							logger.Log(1, "endpoint has changed from ", nodeCfg.Node.Endpoint, " to ", extIP)
							nodeCfg.Node.Endpoint = extIP
							if err := PublishNodeUpdate(&currCommsCfg, &nodeCfg); err != nil {
								logger.Log(0, "could not publish endpoint change")
							}
						}
						intIP, err := getPrivateAddr()
						if err != nil {
							logger.Log(1, "error encountered checking private ip addresses: ", err.Error())
						}
						if nodeCfg.Node.LocalAddress != intIP && intIP != "" {
							logger.Log(1, "local Address has changed from ", nodeCfg.Node.LocalAddress, " to ", intIP)
							nodeCfg.Node.LocalAddress = intIP
							if err := PublishNodeUpdate(&currCommsCfg, &nodeCfg); err != nil {
								logger.Log(0, "could not publish local address change")
							}
						}
					} else if nodeCfg.Node.IsLocal == "yes" && nodeCfg.Node.LocalRange != "" {
						localIP, err := ncutils.GetLocalIP(nodeCfg.Node.LocalRange)
						if err != nil {
							logger.Log(1, "error encountered checking local ip addresses: ", err.Error())
						}
						if nodeCfg.Node.Endpoint != localIP && localIP != "" {
							logger.Log(1, "endpoint has changed from "+nodeCfg.Node.Endpoint+" to ", localIP)
							nodeCfg.Node.Endpoint = localIP
							if err := PublishNodeUpdate(&currCommsCfg, &nodeCfg); err != nil {
								logger.Log(0, "could not publish localip change")
							}
						}
					}
					if err := PingServer(&currCommsCfg); err != nil {
						logger.Log(0, "could not ping server on comms net, ", currCommsCfg.Network, "\n", err.Error())
					} else {
						Hello(&currCommsCfg, &nodeCfg)
					}
				}
			}
		}
	}
}

// PublishNodeUpdates -- saves node and pushes changes to broker
func PublishNodeUpdate(commsCfg, nodeCfg *config.ClientConfig) error {
	if err := config.Write(nodeCfg, nodeCfg.Network); err != nil {
		return err
	}
	data, err := json.Marshal(nodeCfg.Node)
	if err != nil {
		return err
	}
	if err = publish(commsCfg, nodeCfg, fmt.Sprintf("update/%s", nodeCfg.Node.ID), data, 1); err != nil {
		return err
	}
	logger.Log(0, "sent a node update to server for node", nodeCfg.Node.Name, ", ", nodeCfg.Node.ID)
	return nil
}

// Hello -- ping the broker to let server know node it's alive and well
func Hello(commsCfg, nodeCfg *config.ClientConfig) {
	if err := publish(commsCfg, nodeCfg, fmt.Sprintf("ping/%s", nodeCfg.Node.ID), []byte(ncutils.Version), 0); err != nil {
		logger.Log(0, fmt.Sprintf("error publishing ping, %v", err))
		logger.Log(0, "running pull on "+commsCfg.Node.Network+" to reconnect")
		_, err := Pull(commsCfg.Node.Network, true)
		if err != nil {
			logger.Log(0, "could not run pull on "+commsCfg.Node.Network+", error: "+err.Error())
		}
	}
}

// requires the commscfg in which to send traffic over and nodecfg of node that is publish the message
// node cfg is so that the traffic keys of that node may be fetched for encryption
func publish(commsCfg, nodeCfg *config.ClientConfig, dest string, msg []byte, qos byte) error {
	// setup the keys
	trafficPrivKey, err := auth.RetrieveTrafficKey(nodeCfg.Node.Network)
	if err != nil {
		return err
	}

	serverPubKey, err := ncutils.ConvertBytesToKey(nodeCfg.Node.TrafficKeys.Server)
	if err != nil {
		return err
	}

	client := setupMQTT(commsCfg, true)
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
