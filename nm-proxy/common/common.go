package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"

	"github.com/gravitl/netmaker/nm-proxy/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	NmProxyPort = 51722
)

type Conn struct {
	Config ConnConfig
	Proxy  Proxy
}

// ConnConfig is a peer Connection configuration
type ConnConfig struct {

	// Key is a public key of a remote peer
	Key string
	// LocalKey is a public key of a local peer
	LocalKey        string
	LocalWgPort     int
	RemoteProxyIP   net.IP
	RemoteWgPort    int
	RemoteProxyPort int
}
type Config struct {
	Port         int
	BodySize     int
	Addr         string
	RemoteKey    string
	WgInterface  *wg.WGIface
	AllowedIps   []net.IPNet
	PreSharedKey *wgtypes.Key
}

// Proxy -  WireguardProxy proxies
type Proxy struct {
	Ctx    context.Context
	Cancel context.CancelFunc

	Config     Config
	RemoteConn *net.UDPAddr
	LocalConn  net.Conn
}

type RemotePeer struct {
	PeerKey   string
	Interface string
}

var WgIFaceMap = make(map[string]map[string]*Conn)

var PeerKeyHashMap = make(map[string]RemotePeer)

// RunCmd - runs a local command
func RunCmd(command string, printerr bool) (string, error) {
	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command: ", command)
		log.Println(strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}

// API function to interact with netmaker api endpoints. response from endpoint is returned
func API(data interface{}, method, url, authorization string) (*http.Response, error) {
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
	request.Header.Set("requestfrom", "node")
	var httpClient http.Client
	httpClient.Timeout = time.Minute
	return httpClient.Do(request)
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
