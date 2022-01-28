package ncutils

import (
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// MAX_NAME_LENGTH - maximum node name length
const MAX_NAME_LENGTH = 62

// NO_DB_RECORD - error message result
const NO_DB_RECORD = "no result found"

// NO_DB_RECORDS - error record result
const NO_DB_RECORDS = "could not find any records"

// LINUX_APP_DATA_PATH - linux path
const LINUX_APP_DATA_PATH = "/etc/netclient"

// WINDOWS_APP_DATA_PATH - windows path
const WINDOWS_APP_DATA_PATH = "C:\\ProgramData\\Netclient"

// WINDOWS_APP_DATA_PATH - windows path
const WINDOWS_WG_DPAPI_PATH = "C:\\Program Files\\WireGuard\\Data\\Configurations"

// WINDOWS_SVC_NAME - service name
const WINDOWS_SVC_NAME = "netclient"

// NETCLIENT_DEFAULT_PORT - default port
const NETCLIENT_DEFAULT_PORT = 51821

// DEFAULT_GC_PERCENT - garbage collection percent
const DEFAULT_GC_PERCENT = 10

// KEY_SIZE = ideal length for keys
const KEY_SIZE = 2048

// Log - logs a message
func Log(message string) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	log.Println("[netclient]", message)
}

// IsWindows - checks if is windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsMac - checks if is a mac
func IsMac() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux - checks if is linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsLinux - checks if is linux
func IsFreeBSD() bool {
	return runtime.GOOS == "freebsd"
}

// GetWireGuard - checks if wg is installed
func GetWireGuard() string {
	userspace := os.Getenv("WG_QUICK_USERSPACE_IMPLEMENTATION")
	if userspace != "" && (userspace == "boringtun" || userspace == "wireguard-go") {
		return userspace
	}
	return "wg"
}

// IsKernel - checks if running kernel WireGuard
func IsKernel() bool {
	//TODO
	//Replace && true with some config file value
	//This value should be something like kernelmode, which should be 'on' by default.
	return IsLinux() && os.Getenv("WG_QUICK_USERSPACE_IMPLEMENTATION") == ""
}

// IsEmptyRecord - repeat from database
func IsEmptyRecord(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), NO_DB_RECORD) || strings.Contains(err.Error(), NO_DB_RECORDS)
}

//generate an access key value
// GenPass - generates a pass
func GenPass() string {

	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	length := 16
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

// GetPublicIP - gets public ip
func GetPublicIP() (string, error) {

	iplist := []string{"https://ip.client.gravitl.com", "https://ifconfig.me", "https://api.ipify.org", "https://ipinfo.io/ip"}
	endpoint := ""
	var err error
	for _, ipserver := range iplist {
		resp, err := http.Get(ipserver)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := io.ReadAll(resp.Body)
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

// GetMacAddr - get's mac address
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
		keepalive = 0
	}

	for _, peer := range peers {
		endpointString := ""
		if peer.Endpoint != nil && peer.Endpoint.String() != "" {
			endpointString += "Endpoint = " + peer.Endpoint.String()
		}
		newAllowedIps := []string{}
		for _, allowedIP := range peer.AllowedIPs {
			newAllowedIps = append(newAllowedIps, allowedIP.String())
		}
		peersString += fmt.Sprintf(`[Peer]
PublicKey = %s
AllowedIps = %s
PersistentKeepAlive = %s
%s

`,
			peer.PublicKey.String(),
			strings.Join(newAllowedIps, ","),
			strconv.Itoa(int(keepalive)),
			endpointString,
		)
	}
	return peersString, nil
}

// GetLocalIP - gets local ip of machine
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

//GetNetworkIPMask - Pulls the netmask out of the network
func GetNetworkIPMask(networkstring string) (string, string, error) {
	ip, ipnet, err := net.ParseCIDR(networkstring)
	if err != nil {
		return "", "", err
	}
	ipstring := ip.String()
	mask := ipnet.Mask
	maskstring := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	//maskstring := ipnet.Mask.String()
	return ipstring, maskstring, err
}

// GetFreePort - gets free port of machine
func GetFreePort(rangestart int32) (int32, error) {
	if rangestart == 0 {
		rangestart = NETCLIENT_DEFAULT_PORT
	}
	wgclient, err := wgctrl.New()
	if err != nil {
		return 0, err
	}
	defer wgclient.Close()
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

// GetHomeDirWindows - gets home directory in windows
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

// GetNetclientPath - gets netclient path locally
func GetNetclientPath() string {
	if IsWindows() {
		return WINDOWS_APP_DATA_PATH
	} else if IsMac() {
		return "/etc/netclient/"
	} else {
		return LINUX_APP_DATA_PATH
	}
}

// GetNetclientPathSpecific - gets specific netclient config path
func GetNetclientPathSpecific() string {
	if IsWindows() {
		return WINDOWS_APP_DATA_PATH + "\\"
	} else if IsMac() {
		return "/etc/netclient/config/"
	} else {
		return LINUX_APP_DATA_PATH + "/config/"
	}
}

// GetNewIface - Gets the name of the real interface created on Mac
func GetNewIface(dir string) (string, error) {
	files, _ := os.ReadDir(dir)
	var newestFile string
	var newestTime int64 = 0
	var err error
	for _, f := range files {
		fi, err := os.Stat(dir + f.Name())
		if err != nil {
			return "", err
		}
		currTime := fi.ModTime().Unix()
		if currTime > newestTime && strings.Contains(f.Name(), ".sock") {
			newestTime = currTime
			newestFile = f.Name()
		}
	}
	resultArr := strings.Split(newestFile, ".")
	if resultArr[0] == "" {
		err = errors.New("sock file does not exist")
	}
	return resultArr[0], err
}

// GetFileAsString - returns the string contents of a given file
func GetFileAsString(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), err
}

// GetNetclientPathSpecific - gets specific netclient config path
func GetWGPathSpecific() string {
	if IsWindows() {
		return WINDOWS_APP_DATA_PATH + "\\"
	} else {
		return "/etc/wireguard/"
	}
}

// GRPCRequestOpts - gets grps request opts
func GRPCRequestOpts(isSecure string) grpc.DialOption {
	var requestOpts grpc.DialOption
	requestOpts = grpc.WithInsecure()
	if isSecure == "on" {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		requestOpts = grpc.WithTransportCredentials(h2creds)
	}
	return requestOpts
}

// Copy - copies a src file to dest
func Copy(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return errors.New(src + " is not a regular file")
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}
	err = os.Chmod(dst, 0755)
	return err
}

// RunsCmds - runs cmds
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

// FileExists - checks if file exists locally
func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil && strings.Contains(err.Error(), "not a directory") {
		return false
	}
	if err != nil {
		Log("error reading file: " + f + ", " + err.Error())
	}
	return !info.IsDir()
}

// PrintLog - prints log
func PrintLog(message string, loglevel int) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	if loglevel < 2 {
		log.Println("[netclient]", message)
	}
}

// GetSystemNetworks - get networks locally
func GetSystemNetworks() ([]string, error) {
	var networks []string
	files, err := os.ReadDir(GetNetclientPathSpecific())
	if err != nil {
		return networks, err
	}
	for _, f := range files {
		if strings.Contains(f.Name(), "netconfig-") && !strings.Contains(f.Name(), "backup") {
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
	return original[adjustedPosition:]
}

// ShortenString - Brings string down to specified length. Stops names from being too long
func ShortenString(input string, length int) string {
	output := input
	if len(input) > length {
		output = input[0:length]
	}
	return output
}

// DNSFormatString - Formats a string with correct usage for DNS
func DNSFormatString(input string) string {
	reg, err := regexp.Compile("[^a-zA-Z0-9-]+")
	if err != nil {
		Log("error with regex: " + err.Error())
		return ""
	}
	return reg.ReplaceAllString(input, "")
}

// GetHostname - Gets hostname of machine
func GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	if len(hostname) > MAX_NAME_LENGTH {
		hostname = hostname[0:MAX_NAME_LENGTH]
	}
	return hostname
}

// CheckUID - Checks to make sure user has root privileges
func CheckUID() {
	// start our application
	out, err := RunCmd("id -u", true)

	if err != nil {
		log.Fatal(out, err)
	}
	id, err := strconv.Atoi(string(out[:len(out)-1]))

	if err != nil {
		log.Fatal(err)
	}

	if id != 0 {
		log.Fatal("This program must be run with elevated privileges (sudo). This program installs a SystemD service and configures WireGuard and networking rules. Please re-run with sudo/root.")
	}
}

// CheckWG - Checks if WireGuard is installed. If not, exit
func CheckWG() {
	var _, err = exec.LookPath("wg")
	uspace := GetWireGuard()
	if err != nil {
		if uspace == "wg" {
			PrintLog(err.Error(), 0)
			log.Fatal("WireGuard not installed. Please install WireGuard (wireguard-tools) and try again.")
		}
		PrintLog("Running with userspace wireguard: "+uspace, 0)
	} else if uspace != "wg" {
		log.Println("running userspace WireGuard with " + uspace)
	}
}

// ServerAddrSliceContains - sees if a string slice contains a string element
func ServerAddrSliceContains(slice []models.ServerAddr, item models.ServerAddr) bool {
	for _, s := range slice {
		if s.Address == item.Address && s.IsLeader == item.IsLeader {
			return true
		}
	}
	return false
}

// EncryptWithPublicKey encrypts data with public key
func EncryptWithPublicKey(msg []byte, pub *rsa.PublicKey) ([]byte, error) {
	if pub == nil {
		return nil, errors.New("invalid public key when decrypting")
	}
	hash := sha512.New()
	ciphertext, err := rsa.EncryptOAEP(hash, crand.Reader, pub, msg, nil)
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}

// DecryptWithPrivateKey decrypts data with private key
func DecryptWithPrivateKey(ciphertext []byte, priv *rsa.PrivateKey) []byte {
	hash := sha512.New()
	plaintext, err := rsa.DecryptOAEP(hash, crand.Reader, priv, ciphertext, nil)
	if err != nil {
		return nil
	}
	return plaintext
}
