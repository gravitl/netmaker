package logic

import (
	"container/heap"
	"errors"
	"net"
	"net/netip"
	"sync"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
)

var (
	ipPool      map[string]PoolMap
	ipPoolMutex = &sync.RWMutex{}
)

const (
	ipCap = 5000
)

type IpHeap []net.IP

type PoolMap struct {
	V4 *IpHeap
	V6 *IpHeap
}

func (h IpHeap) Len() int { return len(h) }
func (h IpHeap) Less(i, j int) bool {
	addr1, _ := netip.ParseAddr(h[i].String())
	addr2, _ := netip.ParseAddr(h[j].String())
	return addr1.Less(addr2)
}
func (h IpHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *IpHeap) Push(x any) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*h = append(*h, x.(net.IP))
}

func (h *IpHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// ReleaseV4IpForNetwork - release ip back to ip pool after node is deleted, IPV4
func ReleaseV4IpForNetwork(networkName string, ip net.IP) error {
	return releaseIpForNetwork(networkName, ip, "v4")
}

// ReleaseV6IpForNetwork - release ip back to ip pool after node is deleted, IPv6
func ReleaseV6IpForNetwork(networkName string, ip net.IP) error {
	return releaseIpForNetwork(networkName, ip, "v6")
}

// releaseIpForNetwork - release ip back to ip pool after node is deleted
func releaseIpForNetwork(networkName string, ip net.IP, v4v6Type string) error {
	if _, ok := ipPool[networkName]; !ok {
		return errors.New("network does not exist")
	}
	if ip == nil {
		return errors.New("ip is nil, it does not need to return")
	}
	ipPoolMutex.Lock()
	if v4v6Type == "v4" {
		heap.Push(ipPool[networkName].V4, ip)
	} else if v4v6Type == "v6" {
		heap.Push(ipPool[networkName].V6, ip)
	}
	ipPoolMutex.Unlock()
	return nil
}

// AddNetworkToIpPool - add network to ip pool when network is added
func AddNetworkToIpPool(networkName string) error {
	network, err := GetParentNetwork(networkName)
	if err != nil {
		slog.Error("network name is not found ", "Error", networkName, err)
		return err
	}
	pMap := PoolMap{}
	ipv4List := &IpHeap{}
	heap.Init(ipv4List)
	ipv6List := &IpHeap{}
	heap.Init(ipv6List)
	if network.IsIPv4 != "no" {
		//ensure AddressRange is valid
		if _, _, err := net.ParseCIDR(network.AddressRange); err != nil {
			slog.Error("ParseCIDR error ", "Error", networkName, network.AddressRange)
			return err
		}
		net4 := iplib.Net4FromStr(network.AddressRange)
		newAddrs := net4.FirstAddress()

		for {
			heap.Push(ipv4List, newAddrs)
			newAddrs, err = net4.NextIP(newAddrs)
			if err != nil {
				break
			}
		}
	}

	if network.IsIPv6 != "no" {
		// ensure AddressRange is valid
		if _, _, err := net.ParseCIDR(network.AddressRange6); err != nil {
			slog.Error("ParseCIDR error ", "Error", networkName, network.AddressRange)
			return err
		}
		net6 := iplib.Net6FromStr(network.AddressRange6)

		newAddrs, err := net6.NextIP(net6.FirstAddress())
		if err == nil {
			for {
				heap.Push(ipv6List, newAddrs)
				newAddrs, err = net6.NextIP(newAddrs)
				if err != nil {
					break
				}
			}
		}
	}

	pMap.V4 = ipv4List
	pMap.V6 = ipv6List
	ipPoolMutex.Lock()
	ipPool[networkName] = pMap
	ipPoolMutex.Unlock()
	return nil
}

// RemoveNetworkFromIpPool - remove network from ip pool when network is deleted
func RemoveNetworkFromIpPool(networkName string) {
	ipPoolMutex.Lock()
	delete(ipPool, networkName)
	ipPoolMutex.Unlock()
}

// GetUniqueAddress - Allocate unique ipv4 address
func GetUniqueAddress(networkName string) (ip net.IP, err error) {
	if ipPool == nil {
		return ip, errors.New("ip pool is not initialized")
	}
	ipPoolMutex.Lock()
	defer ipPoolMutex.Unlock()

	if _, ok := ipPool[networkName]; !ok {
		return ip, errors.New("network does not exist")
	}

	if len(*ipPool[networkName].V4) == 0 {
		return ip, errors.New("ip v4 pool for network " + networkName + " is empty")
	}

	ip = heap.Pop(ipPool[networkName].V4).(net.IP)

	return
}

// GetUniqueAddress6 - Allocate unique ipv6 address
func GetUniqueAddress6(networkName string) (ip net.IP, err error) {
	if ipPool == nil {
		return ip, errors.New("ip pool is not initialized")
	}
	ipPoolMutex.Lock()
	defer ipPoolMutex.Unlock()

	if _, ok := ipPool[networkName]; !ok {
		return ip, errors.New("network does not exist")
	}

	if len(*ipPool[networkName].V6) == 0 {
		return ip, errors.New("ip v6 pool for network " + networkName + " is empty")
	}

	ip = heap.Pop(ipPool[networkName].V6).(net.IP)

	return
}

// ClearIpPool - set ipPool to nil
func ClearIpPool() {
	ipPool = nil
}

// SetIpPool - set available ip pool for network
func SetIpPool() error {
	logger.Log(0, "start loading ip pool")
	if ipPool == nil {
		ipPool = map[string]PoolMap{}
	}

	currentNetworks, err := GetNetworks()
	if err != nil {
		return err
	}

	for _, v := range currentNetworks {
		pMap := PoolMap{}
		netName := v.NetID

		ipv4List := getAvailableIpV4Pool(&v)
		ipv6List := getAvailableIpV6Pool(&v)

		pMap.V4 = ipv4List
		pMap.V6 = ipv6List

		delete(ipPool, netName)
		ipPool[netName] = pMap
	}
	logger.Log(0, "loading ip pool done")
	return nil
}

func getAvailableIpV4Pool(network *models.Network) *IpHeap {

	ipv4List := &IpHeap{}
	heap.Init(ipv4List)

	if network.IsIPv4 == "no" {
		return ipv4List
	}
	//ensure AddressRange is valid
	if _, _, err := net.ParseCIDR(network.AddressRange); err != nil {
		slog.Debug("UniqueAddress encountered  an error")
		return ipv4List
	}
	net4 := iplib.Net4FromStr(network.AddressRange)
	newAddrs := net4.FirstAddress()

	i := 0
	for {
		if i >= ipCap {
			break
		}
		if IsIPUnique(network.NetID, newAddrs.String(), database.NODES_TABLE_NAME, false) &&
			IsIPUnique(network.NetID, newAddrs.String(), database.EXT_CLIENT_TABLE_NAME, false) {
			heap.Push(ipv4List, newAddrs)
		}

		var err error
		newAddrs, err = net4.NextIP(newAddrs)

		if err != nil {
			break
		}
		i++
	}

	return ipv4List
}

func getAvailableIpV6Pool(network *models.Network) *IpHeap {
	ipv6List := &IpHeap{}
	heap.Init(ipv6List)

	if network.IsIPv6 == "no" {
		return ipv6List
	}

	//ensure AddressRange is valid
	if _, _, err := net.ParseCIDR(network.AddressRange6); err != nil {
		return ipv6List
	}
	net6 := iplib.Net6FromStr(network.AddressRange6)

	newAddrs, err := net6.NextIP(net6.FirstAddress())
	if err != nil {
		return ipv6List
	}

	i := 0
	for {
		if i >= ipCap {
			break
		}
		if IsIPUnique(network.NetID, newAddrs.String(), database.NODES_TABLE_NAME, true) &&
			IsIPUnique(network.NetID, newAddrs.String(), database.EXT_CLIENT_TABLE_NAME, true) {
			heap.Push(ipv6List, newAddrs)
		}

		newAddrs, err = net6.NextIP(newAddrs)

		if err != nil {
			break
		}
		i++
	}

	return ipv6List
}
