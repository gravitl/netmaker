package logic

import (
	"errors"

	"github.com/gravitl/netmaker/models"
)

// CreateHostRelay - creates a host relay
func CreateHostRelay(relay models.HostRelayRequest) (relayHost *models.Host, relayedHosts []models.Host, err error) {

	relayHost, err = GetHost(relay.HostID)
	if err != nil {
		return
	}
	err = ValidateHostRelay(relay)
	if err != nil {
		return
	}
	relayHost.IsRelay = true
	relayHost.ProxyEnabled = true
	relayHost.RelayedHosts = relay.RelayedHosts
	err = UpsertHost(relayHost)
	if err != nil {
		return
	}
	relayedHosts = SetRelayedHosts(true, relay.HostID, relay.RelayedHosts)
	return
}

// SetRelayedHosts - updates the relayed hosts status
func SetRelayedHosts(setRelayed bool, relayHostID string, relayedHostIDs []string) []models.Host {
	var relayedHosts []models.Host
	for _, relayedHostID := range relayedHostIDs {
		host, err := GetHost(relayedHostID)
		if err == nil {
			if setRelayed {
				host.IsRelayed = true
				host.RelayedBy = relayHostID
				host.ProxyEnabled = true
			} else {
				host.IsRelayed = false
				host.RelayedBy = ""
			}
			err = UpsertHost(host)
			if err == nil {
				relayedHosts = append(relayedHosts, *host)
			}
		}
	}
	return relayedHosts
}

// GetRelayedHosts - gets the relayed hosts of a relay host
func GetRelayedHosts(relayHost *models.Host) []models.Host {
	relayedHosts := []models.Host{}

	for _, hostID := range relayHost.RelayedHosts {
		relayedHost, err := GetHost(hostID)
		if err == nil {
			relayedHosts = append(relayedHosts, *relayedHost)
		}
	}
	return relayedHosts
}

func ValidateHostRelay(relay models.HostRelayRequest) error {
	if len(relay.RelayedHosts) == 0 {
		return errors.New("relayed hosts are empty")
	}
	return nil
}

// DeleteHostRelay - removes host as relay
func DeleteHostRelay(relayHostID string) (relayHost *models.Host, relayedHosts []models.Host, err error) {
	relayHost, err = GetHost(relayHostID)
	if err != nil {
		return
	}
	relayedHosts = SetRelayedHosts(false, relayHostID, relayHost.RelayedHosts)
	relayHost.IsRelay = false
	relayHost.RelayedHosts = []string{}
	err = UpsertHost(relayHost)
	if err != nil {
		return
	}
	return
}
