package utils

import (
	"errors"
	"io"
	"net/http"
	"time"
)

// GetPublicIP - gets public ip
func GetPublicIP() (string, error) {

	iplist := []string{"https://ip.client.gravitl.com", "https://ifconfig.me", "https://api.ipify.org", "https://ipinfo.io/ip"}

	//for network, ipService := range global_settings.PublicIPServices {
	//logger.Log(3, "User provided public IP service defined for network", network, "is", ipService)

	// prepend the user-specified service so it's checked first
	//		iplist = append([]string{ipService}, iplist...)
	//}

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
