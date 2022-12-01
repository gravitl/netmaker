package common

import (
	"log"
	"os/exec"
	"strings"

	"github.com/gravitl/netmaker/nm-proxy/models"
)

var IsHostNetwork bool
var IsRelay bool
var IsIngressGateway bool
var IsRelayed bool
var IsServer bool
var InterfaceName string
var BehindNAT bool

var WgIfaceMap = models.WgIfaceConf{
	Iface:   nil,
	PeerMap: make(map[string]*models.Conn),
}

var PeerKeyHashMap = make(map[string]models.RemotePeer)

//var WgIfaceKeyMap = make(map[string]models.RemotePeer)

var RelayPeerMap = make(map[string]map[string]models.RemotePeer)

var ExtClientsWaitTh = make(map[string]models.ExtClientPeer)

var ExtSourceIpMap = make(map[string]models.RemotePeer)

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
