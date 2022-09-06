package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cloverstd/tcping/ping"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/tls"
)

// Checkin  -- go routine that checks for public or local ip changes, publishes changes
//
//	if there are no updates, simply "pings" the server as a checkin
func Checkin(ctx context.Context, wg *sync.WaitGroup) {
	logger.Log(2, "starting checkin goroutine")
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			logger.Log(0, "checkin routine closed")
			return
			//delay should be configuraable -> use cfg.Node.NetworkSettings.DefaultCheckInInterval ??
		case <-time.After(time.Second * 60):
			checkin()
		}
	}
}

func checkin() {
	networks, _ := ncutils.GetSystemNetworks()
	logger.Log(3, "checkin with server(s) for all networks")
	for _, network := range networks {
		var nodeCfg config.ClientConfig
		nodeCfg.Network = network
		nodeCfg.ReadConfig()
		// check for nftables present if on Linux
		if ncutils.IsLinux() {
			if ncutils.IsNFTablesPresent() {
				nodeCfg.Node.FirewallInUse = models.FIREWALL_NFTABLES
			} else {
				nodeCfg.Node.FirewallInUse = models.FIREWALL_IPTABLES
			}
		} else {
			// defaults to iptables for now, may need another default for non-Linux OSes
			nodeCfg.Node.FirewallInUse = models.FIREWALL_IPTABLES
		}
		if nodeCfg.Node.IsStatic != "yes" {
			extIP, err := ncutils.GetPublicIP(nodeCfg.Server.API)
			if err != nil {
				logger.Log(1, "error encountered checking public ip addresses: ", err.Error())
			}
			if nodeCfg.Node.Endpoint != extIP && extIP != "" {
				logger.Log(1, "network:", nodeCfg.Node.Network, "endpoint has changed from ", nodeCfg.Node.Endpoint, " to ", extIP)
				nodeCfg.Node.Endpoint = extIP
				if err := PublishNodeUpdate(&nodeCfg); err != nil {
					logger.Log(0, "network:", nodeCfg.Node.Network, "could not publish endpoint change")
				}
			}
			intIP, err := getPrivateAddr()
			if err != nil {
				logger.Log(1, "network:", nodeCfg.Node.Network, "error encountered checking private ip addresses: ", err.Error())
			}
			if nodeCfg.Node.LocalAddress != intIP && intIP != "" {
				logger.Log(1, "network:", nodeCfg.Node.Network, "local Address has changed from ", nodeCfg.Node.LocalAddress, " to ", intIP)
				nodeCfg.Node.LocalAddress = intIP
				if err := PublishNodeUpdate(&nodeCfg); err != nil {
					logger.Log(0, "Network: ", nodeCfg.Node.Network, " could not publish local address change")
				}
			}
			_ = UpdateLocalListenPort(&nodeCfg)

		} else if nodeCfg.Node.IsLocal == "yes" && nodeCfg.Node.LocalRange != "" {
			localIP, err := ncutils.GetLocalIP(nodeCfg.Node.LocalRange)
			if err != nil {
				logger.Log(1, "network:", nodeCfg.Node.Network, "error encountered checking local ip addresses: ", err.Error())
			}
			if nodeCfg.Node.Endpoint != localIP && localIP != "" {
				logger.Log(1, "network:", nodeCfg.Node.Network, "endpoint has changed from "+nodeCfg.Node.Endpoint+" to ", localIP)
				nodeCfg.Node.Endpoint = localIP
				if err := PublishNodeUpdate(&nodeCfg); err != nil {
					logger.Log(0, "network:", nodeCfg.Node.Network, "could not publish localip change")
				}
			}
		}
		//check version
		if nodeCfg.Node.Version != ncutils.Version {
			nodeCfg.Node.Version = ncutils.Version
			config.Write(&nodeCfg, nodeCfg.Network)
		}
		Hello(&nodeCfg)
		checkCertExpiry(&nodeCfg)
	}
}

// PublishNodeUpdates -- saves node and pushes changes to broker
func PublishNodeUpdate(nodeCfg *config.ClientConfig) error {
	if err := config.Write(nodeCfg, nodeCfg.Network); err != nil {
		return err
	}
	data, err := json.Marshal(nodeCfg.Node)
	if err != nil {
		return err
	}
	if err = publish(nodeCfg, fmt.Sprintf("update/%s", nodeCfg.Node.ID), data, 1); err != nil {
		return err
	}

	logger.Log(0, "network:", nodeCfg.Node.Network, "sent a node update to server for node", nodeCfg.Node.Name, ", ", nodeCfg.Node.ID)
	return nil
}

// Hello -- ping the broker to let server know node it's alive and well
func Hello(nodeCfg *config.ClientConfig) {
	if err := publish(nodeCfg, fmt.Sprintf("ping/%s", nodeCfg.Node.ID), []byte(ncutils.Version), 0); err != nil {
		logger.Log(0, fmt.Sprintf("Network: %s error publishing ping, %v", nodeCfg.Node.Network, err))
		logger.Log(0, "running pull on "+nodeCfg.Node.Network+" to reconnect")
		_, err := Pull(nodeCfg.Node.Network, true)
		if err != nil {
			logger.Log(0, "could not run pull on "+nodeCfg.Node.Network+", error: "+err.Error())
		}
	} else {
		logger.Log(3, "checkin for", nodeCfg.Network, "complete")
	}
}

// node cfg is required  in order to fetch the traffic keys of that node for encryption
func publish(nodeCfg *config.ClientConfig, dest string, msg []byte, qos byte) error {
	// setup the keys
	trafficPrivKey, err := auth.RetrieveTrafficKey(nodeCfg.Node.Network)
	if err != nil {
		return err
	}
	serverPubKey, err := ncutils.ConvertBytesToKey(nodeCfg.Node.TrafficKeys.Server)
	if err != nil {
		return err
	}

	encrypted, err := ncutils.Chunk(msg, serverPubKey, trafficPrivKey)
	if err != nil {
		return err
	}
	if mqclient == nil {
		return errors.New("unable to publish ... no mqclient")
	}
	if token := mqclient.Publish(dest, qos, false, encrypted); !token.WaitTimeout(30*time.Second) || token.Error() != nil {
		logger.Log(0, "could not connect to broker at "+nodeCfg.Server.Server+":"+nodeCfg.Server.MQPort)
		var err error
		if token.Error() == nil {
			err = errors.New("connection timeout")
		} else {
			err = token.Error()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func checkCertExpiry(cfg *config.ClientConfig) error {
	cert, err := tls.ReadCertFromFile(ncutils.GetNetclientServerPath(cfg.Server.Server) + ncutils.GetSeparator() + "client.pem")
	//if cert doesn't exist or will expire within 10 days
	if errors.Is(err, os.ErrNotExist) || cert.NotAfter.Before(time.Now().Add(time.Hour*24*10)) {
		key, err := tls.ReadKeyFromFile(ncutils.GetNetclientPath() + ncutils.GetSeparator() + "client.key")
		if err != nil {
			return err
		}
		return RegisterWithServer(key, cfg)
	}
	if err != nil {
		return err
	}
	return nil
}

func checkBroker(broker string, port string) error {
	if broker == "" {
		return errors.New("error: broker address is blank")
	}
	if port == "" {
		return errors.New("error: broker port is blank")
	}
	_, err := net.LookupIP(broker)
	if err != nil {
		return errors.New("nslookup failed for broker ... check dns records")
	}
	pinger := ping.NewTCPing()
	intPort, err := strconv.Atoi(port)
	if err != nil {
		logger.Log(1, "error converting port to int: "+err.Error())
	}
	pinger.SetTarget(&ping.Target{
		Protocol: ping.TCP,
		Host:     broker,
		Port:     intPort,
		Counter:  3,
		Interval: 1 * time.Second,
		Timeout:  2 * time.Second,
	})
	pingerDone := pinger.Start()
	<-pingerDone
	if pinger.Result().SuccessCounter == 0 {
		return errors.New("unable to connect to broker port ... check netmaker server and firewalls")
	}
	return nil
}
