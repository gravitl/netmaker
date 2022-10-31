package wg

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	DefaultMTU         = 1280
	DefaultWgPort      = 51820
	DefaultWgKeepAlive = 25 * time.Second
)

// WGIface represents a interface instance
type WGIface struct {
	Name      string
	Port      int
	MTU       int
	Device    *wgtypes.Device
	Address   WGAddress
	Interface NetInterface
	mu        sync.Mutex
}

// NetInterface represents a generic network tunnel interface
type NetInterface interface {
	Close() error
}

// WGAddress Wireguard parsed address
type WGAddress struct {
	IP      net.IP
	Network *net.IPNet
}

// NewWGIFace Creates a new Wireguard interface instance
func NewWGIFace(iface string, address string, mtu int) (*WGIface, error) {
	wgIface := &WGIface{
		Name: iface,
		MTU:  mtu,
		mu:   sync.Mutex{},
	}

	wgAddress, err := parseAddress(address)
	if err != nil {
		return wgIface, err
	}

	wgIface.Address = wgAddress
	wgIface.GetWgIface(iface)
	return wgIface, nil
}

func (w *WGIface) GetWgIface(iface string) error {
	wgClient, err := wgctrl.New()
	if err != nil {
		return err
	}
	dev, err := wgClient.Device(iface)
	if err != nil {
		return err
	}

	log.Printf("----> DEVICE: %+v\n", dev)
	w.Device = dev
	w.Port = dev.ListenPort
	return nil
}

// parseAddress parse a string ("1.2.3.4/24") address to WG Address
func parseAddress(address string) (WGAddress, error) {
	ip, network, err := net.ParseCIDR(address)
	if err != nil {
		return WGAddress{}, err
	}
	return WGAddress{
		IP:      ip,
		Network: network,
	}, nil
}

// UpdatePeer updates existing Wireguard Peer or creates a new one if doesn't exist
// Endpoint is optional
func (w *WGIface) UpdatePeer(peerKey string, allowedIps []net.IPNet, keepAlive time.Duration, endpoint *net.UDPAddr, preSharedKey *wgtypes.Key) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	log.Printf("updating interface %s peer %s: endpoint %s ", w.Name, peerKey, endpoint)

	// //parse allowed ips
	// _, ipNet, err := net.ParseCIDR(allowedIps)
	// if err != nil {
	// 	return err
	// }

	peerKeyParsed, err := wgtypes.ParseKey(peerKey)
	if err != nil {
		return err
	}
	peer := wgtypes.PeerConfig{
		PublicKey:                   peerKeyParsed,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  allowedIps,
		PersistentKeepaliveInterval: &keepAlive,
		PresharedKey:                preSharedKey,
		Endpoint:                    endpoint,
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	}
	err = w.configureDevice(config)
	if err != nil {
		return fmt.Errorf("received error \"%v\" while updating peer on interface %s with settings: allowed ips %s, endpoint %s", err, w.Name, allowedIps, endpoint.String())
	}
	return nil
}

// configureDevice configures the wireguard device
func (w *WGIface) configureDevice(config wgtypes.Config) error {
	wg, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wg.Close()

	// validate if device with name exists
	_, err = wg.Device(w.Name)
	if err != nil {
		return err
	}
	log.Printf("got Wireguard device %s\n", w.Name)

	return wg.ConfigureDevice(w.Name, config)
}

// GetListenPort returns the listening port of the Wireguard endpoint
func (w *WGIface) GetListenPort() (*int, error) {
	log.Printf("getting Wireguard listen port of interface %s", w.Name)

	//discover Wireguard current configuration
	wg, err := wgctrl.New()
	if err != nil {
		return nil, err
	}
	defer wg.Close()

	d, err := wg.Device(w.Name)
	if err != nil {
		return nil, err
	}
	log.Printf("got Wireguard device listen port %s, %d", w.Name, d.ListenPort)

	return &d.ListenPort, nil
}
