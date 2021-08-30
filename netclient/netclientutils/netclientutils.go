package netclientutils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const NO_DB_RECORD = "no result found"
const NO_DB_RECORDS = "could not find any records"
const LINUX_APP_DATA_PATH = "/etc/netclient"
const WINDOWS_APP_DATA_PATH = "C:\\ProgramData\\Netclient"
const WINDOWS_SVC_NAME = "netclient"

func Log(message string) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	log.Println("[netclient]", message)
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// == database returned nothing error ==
func IsEmptyRecord(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), NO_DB_RECORD) || strings.Contains(err.Error(), NO_DB_RECORDS)
}

//generate an access key value
func GenPass() string {

	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	length := 16
	charset := "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func GetPublicIP() (string, error) {

	iplist := []string{"http://ip.client.gravitl.com", "https://ifconfig.me", "http://api.ipify.org", "http://ipinfo.io/ip"}
	endpoint := ""
	var err error
	for _, ipserver := range iplist {
		resp, err := http.Get(ipserver)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			endpoint = string(bodyBytes)
			break
		}
	}
	if err == nil && endpoint == "" {
		err = errors.New("public address not found")
	}
	return endpoint, err
}

func GetMacAddr() ([]string, error) {
	ifas, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var as []string
	for _, ifa := range ifas {
		a := ifa.HardwareAddr.String()
		if a != "" {
			as = append(as, a)
		}
	}
	return as, nil
}

func parsePeers(keepalive int32, peers []wgtypes.PeerConfig) (string, error) {
	peersString := ""
	if keepalive <= 0 {
		keepalive = 20
	}
	for _, peer := range peers {
		newAllowedIps := []string{}
		for _, allowedIP := range peer.AllowedIPs {
			newAllowedIps = append(newAllowedIps, allowedIP.String())
		}
		peersString += fmt.Sprintf(`[Peer]
PublicKey = %s
AllowedIps = %s
Endpoint = %s
PersistentKeepAlive = %s

`,
			peer.PublicKey.String(),
			strings.Join(newAllowedIps, ","),
			peer.Endpoint.String(),
			strconv.Itoa(int(keepalive)),
		)
	}
	return peersString, nil
}

func CreateUserSpaceConf(address string, privatekey string, listenPort string, mtu int32, perskeepalive int32, peers []wgtypes.PeerConfig) (string, error) {
	peersString, err := parsePeers(perskeepalive, peers)
	listenPortString := ""
	if mtu <= 0 {
		mtu = 1280
	}
	if listenPort != "" {
		listenPortString += "ListenPort = " + listenPort
	}
	if err != nil {
		return "", err
	}
	config := fmt.Sprintf(`[Interface]
Address = %s
PrivateKey = %s
MTU = %s
%s

%s

`,
		address+"/32",
		privatekey,
		strconv.Itoa(int(mtu)),
		listenPortString,
		peersString)
	return config, nil
}

func GetLocalIP(localrange string) (string, error) {
	_, localRange, err := net.ParseCIDR(localrange)
	if err != nil {
		return "", err
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	var local string
	found := false
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if i.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				if !found {
					ip = v.IP
					local = ip.String()
					found = localRange.Contains(ip)
				}
			case *net.IPAddr:
				if !found {
					ip = v.IP
					local = ip.String()
					found = localRange.Contains(ip)
				}
			}
		}
	}
	if !found || local == "" {
		return "", errors.New("Failed to find local IP in range " + localrange)
	}
	return local, nil
}

func GetFreePort(rangestart int32) (int32, error) {
	wgclient, err := wgctrl.New()
	if err != nil {
		return 0, err
	}
	devices, err := wgclient.Devices()
	if err != nil {
		return 0, err
	}
	var portno int32
	portno = 0
	for x := rangestart; x <= 60000; x++ {
		conflict := false
		for _, i := range devices {
			if int32(i.ListenPort) == x {
				conflict = true
				break
			}
		}
		if conflict {
			continue
		}
		portno = x
		break
	}
	return portno, err
}

// == OS PATH FUNCTIONS ==

func GetHomeDirWindows() string {
	if IsWindows() {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func GetNetclientPath() string {
	if IsWindows() {
		return WINDOWS_APP_DATA_PATH
	} else {
		return LINUX_APP_DATA_PATH
	}
}

func GetNetclientPathSpecific() string {
	if IsWindows() {
		return WINDOWS_APP_DATA_PATH + "\\"
	} else {
		return LINUX_APP_DATA_PATH + "/"
	}
}
