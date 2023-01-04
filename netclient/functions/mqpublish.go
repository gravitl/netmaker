package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cloverstd/tcping/ping"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/metrics"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

var metricsCache = new(sync.Map)

// Checkin  -- go routine that checks for public or local ip changes, publishes changes
//
//	if there are no updates, simply "pings" the server as a checkin
func Checkin(ctx context.Context, wg *sync.WaitGroup) {
	logger.Log(2, "starting checkin goroutine")
	defer wg.Done()
	ticker := time.NewTicker(time.Minute * ncutils.CheckInInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Log(0, "checkin routine closed")
			return
		case <-ticker.C:
			if mqclient != nil && mqclient.IsConnected() {
				checkin()
			} else {
				logger.Log(0, "MQ client is not connected, skipping checkin...")
			}

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
		if nodeCfg.Node.Connected == "yes" {
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
		}
		// check version
		if nodeCfg.Node.Version != ncutils.Version {
			nodeCfg.Node.Version = ncutils.Version
			config.Write(&nodeCfg, nodeCfg.Network)
		}
		Hello(&nodeCfg)
		if nodeCfg.Server.Is_EE && nodeCfg.Node.Connected == "yes" {
			logger.Log(0, "collecting metrics for node", nodeCfg.Node.Name)
			publishMetrics(&nodeCfg)
		}
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
	var checkin models.NodeCheckin
	checkin.Version = ncutils.Version
	checkin.Connected = nodeCfg.Node.Connected
	ip, err := getInterfaces()
	if err != nil {
		logger.Log(0, "failed to retrieve local interfaces", err.Error())
	} else {
		nodeCfg.Node.Interfaces = *ip
		config.Write(nodeCfg, nodeCfg.Network)
	}
	checkin.Ifaces = nodeCfg.Node.Interfaces
	data, err := json.Marshal(checkin)
	if err != nil {
		logger.Log(0, "unable to marshal checkin data", err.Error())
		return
	}
	if err := publish(nodeCfg, fmt.Sprintf("ping/%s", nodeCfg.Node.ID), data, 0); err != nil {
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

// publishMetrics - publishes the metrics of a given nodecfg
func publishMetrics(nodeCfg *config.ClientConfig) {
	token, err := Authenticate(nodeCfg)
	if err != nil {
		logger.Log(1, "failed to authenticate when publishing metrics", err.Error())
		return
	}
	url := fmt.Sprintf("https://%s/api/nodes/%s/%s", nodeCfg.Server.API, nodeCfg.Network, nodeCfg.Node.ID)
	response, err := API("", http.MethodGet, url, token)
	if err != nil {
		logger.Log(1, "failed to read from server during metrics publish", err.Error())
		return
	}
	if response.StatusCode != http.StatusOK {
		bytes, err := io.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		logger.Log(0, fmt.Sprintf("%s %s", string(bytes), err.Error()))
		return
	}
	defer response.Body.Close()
	var nodeGET models.NodeGet
	if err := json.NewDecoder(response.Body).Decode(&nodeGET); err != nil {
		logger.Log(0, "failed to decode node when running metrics update", err.Error())
		return
	}

	collected, err := metrics.Collect(nodeCfg.Node.Interface, nodeGET.PeerIDs)
	if err != nil {
		logger.Log(0, "failed metric collection for node", nodeCfg.Node.Name, err.Error())
		return
	}
	collected.Network = nodeCfg.Node.Network
	collected.NodeName = nodeCfg.Node.Name
	collected.NodeID = nodeCfg.Node.ID
	collected.IsServer = "no"
	data, err := json.Marshal(collected)
	if err != nil {
		logger.Log(0, "something went wrong when marshalling metrics data for node", nodeCfg.Node.Name, err.Error())
	}

	if err = publish(nodeCfg, fmt.Sprintf("metrics/%s", nodeCfg.Node.ID), data, 1); err != nil {
		logger.Log(0, "error occurred during publishing of metrics on node", nodeCfg.Node.Name, err.Error())
		logger.Log(0, "aggregating metrics locally until broker connection re-established")
		val, ok := metricsCache.Load(nodeCfg.Node.ID)
		if !ok {
			metricsCache.Store(nodeCfg.Node.ID, data)
		} else {
			var oldMetrics models.Metrics
			err = json.Unmarshal(val.([]byte), &oldMetrics)
			if err == nil {
				for k := range oldMetrics.Connectivity {
					currentMetric := collected.Connectivity[k]
					if currentMetric.Latency == 0 {
						currentMetric.Latency = oldMetrics.Connectivity[k].Latency
					}
					currentMetric.Uptime += oldMetrics.Connectivity[k].Uptime
					currentMetric.TotalTime += oldMetrics.Connectivity[k].TotalTime
					collected.Connectivity[k] = currentMetric
				}
				newData, err := json.Marshal(collected)
				if err == nil {
					metricsCache.Store(nodeCfg.Node.ID, newData)
				}
			}
		}
	} else {
		metricsCache.Delete(nodeCfg.Node.ID)
		logger.Log(0, "published metrics for node", nodeCfg.Node.Name)
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
