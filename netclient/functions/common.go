package functions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// LINUX_APP_DATA_PATH - linux path
const LINUX_APP_DATA_PATH = "/etc/netmaker"

// HTTP_TIMEOUT - timeout in seconds for http requests
const HTTP_TIMEOUT = 30

// HTTPClient - http client to be reused by all
var HTTPClient http.Client

// SetHTTPClient -sets http client with sane default
func SetHTTPClient() {
	HTTPClient = http.Client{
		Timeout: HTTP_TIMEOUT * time.Second,
	}
}

// ListPorts - lists ports of WireGuard devices
func ListPorts() error {
	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgclient.Close()
	devices, err := wgclient.Devices()
	if err != nil {
		return err
	}
	fmt.Println("Here are your ports:")
	for _, i := range devices {
		fmt.Println(i.ListenPort)
	}
	return err
}

func getPrivateAddr() (string, error) {

	var local string
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()

		localAddr := conn.LocalAddr().(*net.UDPAddr)
		localIP := localAddr.IP
		local = localIP.String()
	}
	if local == "" {
		local, err = getPrivateAddrBackup()
	}

	if local == "" {
		err = errors.New("could not find local ip")
	}

	return local, err
}

func getPrivateAddrBackup() (string, error) {
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
					found = true
				}
			case *net.IPAddr:
				if !found {
					ip = v.IP
					local = ip.String()
					found = true
				}
			}
		}
	}
	if !found {
		err := errors.New("local ip address not found")
		return "", err
	}
	return local, err
}

// GetNode - gets node locally
func GetNode(network string) models.Node {

	modcfg, err := config.ReadConfig(network)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	return modcfg.Node
}

// Uninstall - uninstalls networks from client
func Uninstall() error {
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		logger.Log(1, "unable to retrieve networks: ", err.Error())
		logger.Log(1, "continuing uninstall without leaving networks")
	} else {
		for _, network := range networks {
			err = LeaveNetwork(network)
			if err != nil {
				logger.Log(1, "encounter issue leaving network", network, ":", err.Error())
			}
		}
	}
	err = nil

	// clean up OS specific stuff
	if ncutils.IsWindows() {
		daemon.CleanupWindows()
	} else if ncutils.IsMac() {
		daemon.CleanupMac()
	} else if ncutils.IsLinux() {
		daemon.CleanupLinux()
	} else if ncutils.IsFreeBSD() {
		daemon.CleanupFreebsd()
	} else if !ncutils.IsKernel() {
		logger.Log(1, "manual cleanup required")
	}

	return err
}

// LeaveNetwork - client exits a network
func LeaveNetwork(network string) error {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	logger.Log(2, "deleting node from server")
	if err := deleteNodeFromServer(cfg); err != nil {
		logger.Log(0, "error deleting node from server", err.Error())
	}
	logger.Log(2, "deleting wireguard interface")
	if err := deleteLocalNetwork(cfg); err != nil {
		logger.Log(0, "error deleting wireguard interface", err.Error())
	}
	logger.Log(2, "deleting configuration files")
	if err := WipeLocal(cfg); err != nil {
		logger.Log(0, "error deleting local network files", err.Error())
	}
	logger.Log(2, "removing dns entries")
	if err := removeHostDNS(cfg.Node.Interface, ncutils.IsWindows()); err != nil {
		logger.Log(0, "failed to delete dns entries for", cfg.Node.Interface, err.Error())
	}
	logger.Log(2, "deleting broker keys as required")
	if !brokerInUse(cfg.Server.Server) {
		if err := deleteBrokerFiles(cfg.Server.Server); err != nil {
			logger.Log(0, "failed to deleter certs for", cfg.Server.Server, err.Error())
		}
	}
	logger.Log(2, "restarting daemon")
	return daemon.Restart()
}

func brokerInUse(broker string) bool {
	networks, _ := ncutils.GetSystemNetworks()
	for _, net := range networks {
		cfg := config.ClientConfig{}
		cfg.Network = net
		cfg.ReadConfig()
		if cfg.Server.Server == broker {
			return true
		}
	}
	return false
}

func deleteBrokerFiles(broker string) error {
	dir := ncutils.GetNetclientServerPath(broker)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return nil
}

func deleteNodeFromServer(cfg *config.ClientConfig) error {
	node := cfg.Node
	if node.IsServer == "yes" {
		return errors.New("attempt to delete server node ... not permitted")
	}
	token, err := Authenticate(cfg)
	if err != nil {
		return fmt.Errorf("unable to authenticate %w", err)
	}
	url := "https://" + cfg.Server.API + "/api/nodes/" + cfg.Network + "/" + cfg.Node.ID
	response, err := API("", http.MethodDelete, url, token)
	if err != nil {
		return fmt.Errorf("error deleting node on server: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		bodybytes, _ := io.ReadAll(response.Body)
		defer response.Body.Close()
		return fmt.Errorf("error deleting node from network %s on server %s %s", cfg.Network, response.Status, string(bodybytes))
	}
	return nil
}

func deleteLocalNetwork(cfg *config.ClientConfig) error {
	wgClient, wgErr := wgctrl.New()
	if wgErr != nil {
		return wgErr
	}
	removeIface := cfg.Node.Interface
	queryAddr := cfg.Node.PrimaryAddress()
	if ncutils.IsMac() {
		var macIface string
		macIface, wgErr = local.GetMacIface(queryAddr)
		if wgErr == nil && removeIface != "" {
			removeIface = macIface
		}
	}
	dev, devErr := wgClient.Device(removeIface)
	if devErr != nil {
		return fmt.Errorf("error flushing routes %w", devErr)
	}
	local.FlushPeerRoutes(removeIface, queryAddr, dev.Peers[:])
	_, cidr, cidrErr := net.ParseCIDR(cfg.NetworkSettings.AddressRange)
	if cidrErr != nil {
		return fmt.Errorf("error flushing routes %w", cidrErr)
	}
	local.RemoveCIDRRoute(removeIface, queryAddr, cidr)
	return nil
}

// DeleteInterface - delete an interface of a network
func DeleteInterface(ifacename string, postdown string) error {
	return wireguard.RemoveConf(ifacename, true)
}

// WipeLocal - wipes local instance
func WipeLocal(cfg *config.ClientConfig) error {
	if err := wireguard.RemoveConf(cfg.Node.Interface, true); err == nil {
		logger.Log(1, "network:", cfg.Node.Network, "removed WireGuard interface: ", cfg.Node.Interface)
	} else if strings.Contains(err.Error(), "does not exist") {
		err = nil
	}
	dir := ncutils.GetNetclientPathSpecific()
	fail := false
	files, err := filepath.Glob(dir + "*" + cfg.Node.Network)
	if err != nil {
		logger.Log(0, "no matching files", err.Error())
		fail = true
	}
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			logger.Log(0, "failed to delete file", file, err.Error())
			fail = true
		}
	}

	if cfg.Node.Interface != "" {
		if ncutils.FileExists(dir + cfg.Node.Interface + ".conf") {
			if err := os.Remove(dir + cfg.Node.Interface + ".conf"); err != nil {
				log.Println("error removing .conf:")
				log.Println(err.Error())
				fail = true
			}
		}
	}
	if fail {
		return errors.New("not all files were deleted")
	}
	return nil
}

// GetNetmakerPath - gets netmaker path locally
func GetNetmakerPath() string {
	return LINUX_APP_DATA_PATH
}

// API function to interact with netmaker api endpoints. response from endpoint is returned
func API(data any, method, url, authorization string) (*http.Response, error) {
	var request *http.Request
	var err error
	if data != "" {
		payload, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("error encoding data %w", err)
		}
		request, err = http.NewRequest(method, url, bytes.NewBuffer(payload))
		if err != nil {
			return nil, fmt.Errorf("error creating http request %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
	} else {
		request, err = http.NewRequest(method, url, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating http request %w", err)
		}
	}
	if authorization != "" {
		request.Header.Set("authorization", "Bearer "+authorization)
	}
	return HTTPClient.Do(request)
}

// Authenticate authenticates with api to permit subsequent interactions with the api
func Authenticate(cfg *config.ClientConfig) (string, error) {

	pass, err := os.ReadFile(ncutils.GetNetclientPathSpecific() + "secret-" + cfg.Network)
	if err != nil {
		return "", fmt.Errorf("could not read secrets file %w", err)
	}
	data := models.AuthParams{
		MacAddress: cfg.Node.MacAddress,
		ID:         cfg.Node.ID,
		Password:   string(pass),
	}
	url := "https://" + cfg.Server.API + "/api/nodes/adm/" + cfg.Network + "/authenticate"
	response, err := API(data, http.MethodPost, url, "")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		bodybytes, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("failed to authenticate %s %s", response.Status, string(bodybytes))
	}
	resp := models.SuccessResponse{}
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return "", fmt.Errorf("error decoding respone %w", err)
	}
	tokenData := resp.Response.(map[string]interface{})
	token := tokenData["AuthToken"]
	return token.(string), nil
}

// RegisterWithServer calls the register endpoint with privatekey and commonname - api returns ca and client certificate
func SetServerInfo(cfg *config.ClientConfig) error {
	cfg, err := config.ReadConfig(cfg.Network)
	if err != nil {
		return err
	}
	url := "https://" + cfg.Server.API + "/api/server/getserverinfo"
	logger.Log(1, "server at "+url)

	token, err := Authenticate(cfg)
	if err != nil {
		return err
	}
	response, err := API("", http.MethodGet, url, token)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return errors.New(response.Status)
	}
	var resp models.ServerConfig
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return errors.New("unmarshal cert error " + err.Error())
	}

	// set broker information on register
	cfg.Server.Server = resp.Server
	cfg.Server.MQPort = resp.MQPort

	if err = config.ModServerConfig(&cfg.Server, cfg.Node.Network); err != nil {
		logger.Log(0, "error overwriting config with broker information: "+err.Error())
	}

	return nil
}

func informPortChange(node *models.Node) {
	if node.ListenPort == 0 {
		logger.Log(0, "network:", node.Network, "UDP hole punching enabled for node", node.Name)
	} else {
		logger.Log(0, "network:", node.Network, "node", node.Name, "is using port", strconv.Itoa(int(node.ListenPort)))
	}
}
