package ncutils

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const NO_DB_RECORD = "no result found"
const NO_DB_RECORDS = "could not find any records"
const LINUX_APP_DATA_PATH = "/etc/netclient"
const WINDOWS_APP_DATA_PATH = "C:\\ProgramData\\Netclient"
const WINDOWS_SVC_NAME = "netclient"
const NETCLIENT_DEFAULT_PORT = 51821
const DEFAULT_GC_PERCENT = 10

func Log(message string) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	log.Println("[netclient]", message)
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func IsMac() bool {
	return runtime.GOOS == "darwin"
}

func IsLinux() bool {
	return runtime.GOOS == "linux"
}

func GetWireGuard() string {
	userspace := os.Getenv("WG_QUICK_USERSPACE_IMPLEMENTATION")
	if userspace != "" && (userspace == "boringtun" || userspace == "wireguard-go") {
		return userspace
	}
	return "wg"
}

func IsKernel() bool {
	//TODO
	//Replace && true with some config file value
	//This value should be something like kernelmode, which should be 'on' by default.
	return IsLinux() && os.Getenv("WG_QUICK_USERSPACE_IMPLEMENTATION") == ""
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
	if rangestart == 0 {
		rangestart = NETCLIENT_DEFAULT_PORT
	}
	wgclient, err := wgctrl.New()
	if err != nil {
		return 0, err
	}
	devices, err := wgclient.Devices()
	if err != nil {
		return 0, err
	}
	for x := rangestart; x <= 65535; x++ {
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
		return int32(x), nil
	}
	return rangestart, err
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
	} else if IsMac() {
		return "/etc/netclient/"
	} else {
		return LINUX_APP_DATA_PATH
	}
}

func GetNetclientPathSpecific() string {
	if IsWindows() {
		return WINDOWS_APP_DATA_PATH + "\\"
	} else if IsMac() {
		return "/etc/netclient/config/"
	} else {
		return LINUX_APP_DATA_PATH + "/config/"
	}
}

func GRPCRequestOpts(isSecure string) grpc.DialOption {
	var requestOpts grpc.DialOption
	requestOpts = grpc.WithInsecure()
	if isSecure == "on" {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		requestOpts = grpc.WithTransportCredentials(h2creds)
	}
	return requestOpts
}

func Copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, errors.New(src + " is not a regular file")
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	err = os.Chmod(dst, 0755)
	if err != nil {
		log.Println(err)
	}
	return nBytes, err
}

func RunCmd(command string, printerr bool) (string, error) {
	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command:", command)
		log.Println(strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}

func RunCmds(commands []string, printerr bool) error {
	var err error
	for _, command := range commands {
		args := strings.Fields(command)
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil && printerr {
			log.Println("error running command:", command)
			log.Println(strings.TrimSuffix(string(out), "\n"))
		}
	}
	return err
}

func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func PrintLog(message string, loglevel int) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	if loglevel < 2 {
		log.Println("[netclient]", message)
	}
}

func GetSystemNetworks() ([]string, error) {
	var networks []string
	files, err := ioutil.ReadDir(GetNetclientPathSpecific())
	if err != nil {
		return networks, err
	}
	for _, f := range files {
		if strings.Contains(f.Name(), "netconfig-") {
			networkname := stringAfter(f.Name(), "netconfig-")
			networks = append(networks, networkname)
		}
	}
	return networks, err
}

func stringAfter(original string, substring string) string {
	position := strings.LastIndex(original, substring)
	if position == -1 {
		return ""
	}
	adjustedPosition := position + len(substring)

	if adjustedPosition >= len(original) {
		return ""
	}
	return original[adjustedPosition:len(original)]
}