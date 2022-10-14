// package for logicing client and server code
package logic

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// IsBase64 - checks if a string is in base64 format
// This is used to validate public keys (make sure they're base64 encoded like all public keys should be).
func IsBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

// CheckEndpoint - checks if an endpoint is valid
func CheckEndpoint(endpoint string) bool {
	endpointarr := strings.Split(endpoint, ":")
	return len(endpointarr) == 2
}

// FileExists - checks if local file exists
func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// IsAddressInCIDR - util to see if an address is in a cidr or not
func IsAddressInCIDR(address, cidr string) bool {
	var _, currentCIDR, cidrErr = net.ParseCIDR(cidr)
	if cidrErr != nil {
		return false
	}
	var addrParts = strings.Split(address, ".")
	var addrPartLength = len(addrParts)
	if addrPartLength != 4 {
		return false
	} else {
		if addrParts[addrPartLength-1] == "0" ||
			addrParts[addrPartLength-1] == "255" {
			return false
		}
	}
	ip, _, err := net.ParseCIDR(fmt.Sprintf("%s/32", address))
	if err != nil {
		return false
	}
	return currentCIDR.Contains(ip)
}

// SetNetworkNodesLastModified - sets the network nodes last modified
func SetNetworkNodesLastModified(networkName string) error {

	timestamp := time.Now().Unix()

	network, err := GetParentNetwork(networkName)
	if err != nil {
		return err
	}
	network.NodesLastModified = timestamp
	data, err := json.Marshal(&network)
	if err != nil {
		return err
	}
	err = database.Insert(networkName, string(data), database.NETWORKS_TABLE_NAME)
	if err != nil {
		return err
	}
	return nil
}

// GenerateCryptoString - generates random string of n length
func GenerateCryptoString(n int) (string, error) {
	const chars = "123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	ret := make([]byte, n)
	for i := range ret {
		num, err := crand.Int(crand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		ret[i] = chars[num.Int64()]
	}

	return string(ret), nil
}

// RandomString - returns a random string in a charset
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// == Private Methods ==

func setIPForwardingLinux() error {
	out, err := ncutils.RunCmd("sysctl net.ipv4.ip_forward", true)
	if err != nil {
		logger.Log(0, "WARNING: Error encountered setting ip forwarding. This can break functionality.")
		return err
	} else {
		s := strings.Fields(string(out))
		if s[2] != "1" {
			_, err = ncutils.RunCmd("sysctl -w net.ipv4.ip_forward=1", true)
			if err != nil {
				logger.Log(0, "WARNING: Error encountered setting ip forwarding. You may want to investigate this.")
				return err
			}
		}
	}
	return nil
}

// StringSliceContains - sees if a string slice contains a string element
func StringSliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// == private ==

// sets the network server peers of a given node
func setNetworkServerPeers(serverNode *models.Node) {
	if currentPeersList, err := getSystemPeers(serverNode); err == nil {
		if currentPeersList == nil {
			currentPeersList = make(map[string]string)
		}
		if database.SetPeers(currentPeersList, serverNode.Network) {
			logger.Log(1, "set new peers on network", serverNode.Network)
		}
	} else {
		logger.Log(1, "could not set peers on network", serverNode.Network, ":", err.Error())
	}
}

// ShouldPublishPeerPorts - Gets ports from iface, sets, and returns true if they are different
func ShouldPublishPeerPorts(serverNode *models.Node) bool {
	if currentPeersList, err := getSystemPeers(serverNode); err == nil {
		if database.SetPeers(currentPeersList, serverNode.Network) {
			logger.Log(1, "set new peers on network", serverNode.Network)
			return true
		}
	}
	return false
}

// NormalCIDR - returns the first address of CIDR
func NormalizeCIDR(address string) (string, error) {
	ip, IPNet, err := net.ParseCIDR(address)
	if err != nil {
		return "", err
	}
	if ip.To4() == nil {
		net6 := iplib.Net6FromStr(IPNet.String())
		IPNet.IP = net6.FirstAddress()
	} else {
		net4 := iplib.Net4FromStr(IPNet.String())
		IPNet.IP = net4.NetworkAddress()
	}
	return IPNet.String(), nil
}

func getNetworkProtocols(cidrs []string) (bool, bool) {
	ipv4 := false
	ipv6 := false
	for _, cidr := range cidrs {
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ip.To4() == nil {
			ipv6 = true
		} else {
			ipv4 = true
		}
	}
	return ipv4, ipv6
}

// StringDifference - returns the elements in `a` that aren't in `b`.
func StringDifference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

// CheckIfFileExists - checks if file exists or not in the given path
func CheckIfFileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}
