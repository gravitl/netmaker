package ncutils

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/global_settings"
)

var (
	// Version - version of the netclient
	Version = "dev"
)

// MAX_NAME_LENGTH - maximum node name length
const MAX_NAME_LENGTH = 62

// NO_DB_RECORD - error message result
const NO_DB_RECORD = "no result found"

// NO_DB_RECORDS - error record result
const NO_DB_RECORDS = "could not find any records"

// LINUX_APP_DATA_PATH - linux path
const LINUX_APP_DATA_PATH = "/etc/netclient"

// MAC_APP_DATA_PATH - mac path
const MAC_APP_DATA_PATH = "/Applications/Netclient"

// WINDOWS_APP_DATA_PATH - windows path
const WINDOWS_APP_DATA_PATH = "C:\\Program Files (x86)\\Netclient"

// WINDOWS_SVC_NAME - service name
const WINDOWS_SVC_NAME = "netclient"

// NETCLIENT_DEFAULT_PORT - default port
const NETCLIENT_DEFAULT_PORT = 51821

// DEFAULT_GC_PERCENT - garbage collection percent
const DEFAULT_GC_PERCENT = 10

// KEY_SIZE = ideal length for keys
const KEY_SIZE = 2048

// constants for random strings
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// SetVersion -- set netclient version for use by other packages
func SetVersion(ver string) {
	Version = ver
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

// IsFreeBSD - checks if is freebsd
func IsFreeBSD() bool {
	return runtime.GOOS == "freebsd"
}

// HasWGQuick - checks if WGQuick command is present
func HasWgQuick() bool {
	cmd, err := exec.LookPath("wg-quick")
	return err == nil && cmd != ""
}

// GetWireGuard - checks if wg is installed
func GetWireGuard() string {
	userspace := os.Getenv("WG_QUICK_USERSPACE_IMPLEMENTATION")
	if userspace != "" && (userspace == "boringtun" || userspace == "wireguard-go") {
		return userspace
	}
	return "wg"
}

// IsNFTablesPresent - returns true if nftables is present, false otherwise.
// Does not consider OS, up to the caller to determine if the OS supports nftables/whether this check is valid.
func IsNFTablesPresent() bool {
	found := false
	_, err := exec.LookPath("nft")
	if err == nil {
		found = true
	}
	return found
}

// IsIPTablesPresent - returns true if iptables is present, false otherwise
// Does not consider OS, up to the caller to determine if the OS supports iptables/whether this check is valid.
func IsIPTablesPresent() bool {
	found := false
	_, err := exec.LookPath("iptables")
	if err == nil {
		found = true
	}
	return found
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

// GetPublicIP - gets public ip
func GetPublicIP(api string) (string, error) {

	iplist := []string{"https://ip.client.gravitl.com", "https://ifconfig.me", "https://api.ipify.org", "https://ipinfo.io/ip"}

	for network, ipService := range global_settings.PublicIPServices {
		logger.Log(3, "User provided public IP service defined for network", network, "is", ipService)

		// prepend the user-specified service so it's checked first
		iplist = append([]string{ipService}, iplist...)
	}
	if api != "" {
		api = "https://" + api + "/api/getip"
		iplist = append([]string{api}, iplist...)
	}

	endpoint := ""
	var err error
	for _, ipserver := range iplist {
		client := &http.Client{
			Timeout: time.Second * 10,
		}
		resp, err := client.Get(ipserver)
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

// GetNetworkIPMask - Pulls the netmask out of the network
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
	addr := net.UDPAddr{}
	if rangestart == 0 {
		rangestart = NETCLIENT_DEFAULT_PORT
	}
	for x := rangestart; x <= 65535; x++ {
		addr.Port = int(x)
		conn, err := net.ListenUDP("udp", &addr)
		if err != nil {
			continue
		}
		defer conn.Close()
		return x, nil
	}
	return rangestart, errors.New("no free ports")
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
		return MAC_APP_DATA_PATH
	} else {
		return LINUX_APP_DATA_PATH
	}
}

// GetSeparator - gets the separator for OS
func GetSeparator() string {
	if IsWindows() {
		return "\\"
	} else {
		return "/"
	}
}

// GetFileWithRetry - retry getting file X number of times before failing
func GetFileWithRetry(path string, retryCount int) ([]byte, error) {
	var data []byte
	var err error
	for count := 0; count < retryCount; count++ {
		data, err = os.ReadFile(path)
		if err == nil {
			return data, err
		} else {
			logger.Log(1, "failed to retrieve file ", path, ", retrying...")
			time.Sleep(time.Second >> 2)
		}
	}
	return data, err
}

// GetNetclientServerPath - gets netclient server path
func GetNetclientServerPath(server string) string {
	if IsWindows() {
		return WINDOWS_APP_DATA_PATH + "\\" + server + "\\"
	} else if IsMac() {
		return MAC_APP_DATA_PATH + "/" + server + "/"
	} else {
		return LINUX_APP_DATA_PATH + "/" + server
	}
}

// GetNetclientPathSpecific - gets specific netclient config path
func GetNetclientPathSpecific() string {
	if IsWindows() {
		return WINDOWS_APP_DATA_PATH + "\\"
	} else if IsMac() {
		return MAC_APP_DATA_PATH + "/config/"
	} else {
		return LINUX_APP_DATA_PATH + "/config/"
	}
}

func CheckIPAddress(ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("ip address %s is invalid", ip)
	}
	return nil
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
			logger.Log(0, "error running command:", command)
			logger.Log(0, strings.TrimSuffix(string(out), "\n"))
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
		logger.Log(0, "error reading file: "+f+", "+err.Error())
	}
	return !info.IsDir()
}

// GetSystemNetworks - get networks locally
func GetSystemNetworks() ([]string, error) {
	var networks []string
	files, err := filepath.Glob(GetNetclientPathSpecific() + "netconfig-*")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		//don't want files such as *.bak, *.swp
		if filepath.Ext(file) != "" {
			continue
		}
		file := filepath.Base(file)
		temp := strings.Split(file, "-")
		networks = append(networks, strings.Join(temp[1:], "-"))
	}
	return networks, nil
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
		logger.Log(0, "error with regex: "+err.Error())
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

// CheckFirewall - checks if iptables of nft install, if not exit
func CheckFirewall() {
	if !IsIPTablesPresent() && !IsNFTablesPresent() {
		log.Fatal("neither iptables nor nft is installed - please install one or the other and try again")
	}
}

// CheckWG - Checks if WireGuard is installed. If not, exit
func CheckWG() {
	uspace := GetWireGuard()
	if !HasWG() {
		if uspace == "wg" {
			log.Fatal("WireGuard not installed. Please install WireGuard (wireguard-tools) and try again.")
		}
		logger.Log(0, "running with userspace wireguard: ", uspace)
	} else if uspace != "wg" {
		logger.Log(0, "running userspace WireGuard with ", uspace)
	}
}

// HasWG - returns true if wg command exists
func HasWG() bool {
	var _, err = exec.LookPath("wg")
	return err == nil
}

// ConvertKeyToBytes - util to convert a key to bytes to use elsewhere
func ConvertKeyToBytes(key *[32]byte) ([]byte, error) {
	var buffer bytes.Buffer
	var enc = gob.NewEncoder(&buffer)
	if err := enc.Encode(key); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// ConvertBytesToKey - util to convert bytes to a key to use elsewhere
func ConvertBytesToKey(data []byte) (*[32]byte, error) {
	var buffer = bytes.NewBuffer(data)
	var dec = gob.NewDecoder(buffer)
	var result = new([32]byte)
	var err = dec.Decode(result)
	if err != nil {
		return nil, err
	}
	return result, err
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

// MakeRandomString - generates a random string of len n
func MakeRandomString(n int) string {
	const validChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, n)
	if _, err := rand.Reader.Read(result); err != nil {
		return ""
	}
	for i, b := range result {
		result[i] = validChars[b%byte(len(validChars))]
	}
	return string(result)
}

func GetIPNetFromString(ip string) (net.IPNet, error) {
	var ipnet *net.IPNet
	var err error
	// parsing as a CIDR first. If valid CIDR, append
	if _, cidr, err := net.ParseCIDR(ip); err == nil {
		ipnet = cidr
	} else { // parsing as an IP second. If valid IP, check if ipv4 or ipv6, then append
		if iplib.Version(net.ParseIP(ip)) == 4 {
			ipnet = &net.IPNet{
				IP:   net.ParseIP(ip),
				Mask: net.CIDRMask(32, 32),
			}
		} else if iplib.Version(net.ParseIP(ip)) == 6 {
			ipnet = &net.IPNet{
				IP:   net.ParseIP(ip),
				Mask: net.CIDRMask(128, 128),
			}
		}
	}
	if ipnet == nil {
		err = errors.New(ip + " is not a valid ip or cidr")
		return net.IPNet{}, err
	}
	return *ipnet, err
}

// ModPort - Change Node Port if UDP Hole Punching or ListenPort is not free
func ModPort(node *models.Node) error {
	var err error
	if node.UDPHolePunch == "yes" {
		node.ListenPort = 0
	} else {
		node.ListenPort, err = GetFreePort(node.ListenPort)
	}
	return err
}
