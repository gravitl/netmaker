package ips

import (
	"fmt"
	"strings"

	"github.com/seancfoley/ipaddress-go/ipaddr"
)

// GetFirstAddr - gets the first valid address in a given IPv4 CIDR
func GetFirstAddr(cidr4 string) (*ipaddr.IPAddress, error) {
	currentCidr := ipaddr.NewIPAddressString(cidr4).GetAddress()
	if !currentCidr.IsIPv4() {
		return nil, fmt.Errorf("invalid IPv4 CIDR provided to GetFirstAddr")
	}
	lower := currentCidr.GetLower()
	ipParts := strings.Split(lower.GetNetIPAddr().IP.String(), ".")
	if ipParts[len(ipParts)-1] == "0" {
		lower = lower.Increment(1)
	}
	return lower, nil
}

// GetLastAddr - gets the last valid address in a given IPv4 CIDR
func GetLastAddr(cidr4 string) (*ipaddr.IPAddress, error) {
	currentCidr := ipaddr.NewIPAddressString(cidr4).GetAddress()
	if !currentCidr.IsIPv4() {
		return nil, fmt.Errorf("invalid IPv4 CIDR provided to GetLastAddr")
	}
	upper := currentCidr.GetUpper()
	ipParts := strings.Split(upper.GetNetIPAddr().IP.String(), ".")
	if ipParts[len(ipParts)-1] == "255" {
		upper = upper.Increment(-1)
	}
	return upper, nil
}

// GetFirstAddr6 - gets the first valid IPv6 address in a given IPv6 CIDR
func GetFirstAddr6(cidr6 string) (*ipaddr.IPAddress, error) {
	currentCidr := ipaddr.NewIPAddressString(cidr6).GetAddress()
	if !currentCidr.IsIPv6() {
		return nil, fmt.Errorf("invalid IPv6 CIDR provided to GetFirstAddr6")
	}
	lower := currentCidr.GetLower()
	return lower, nil
}

// GetLastAddr6 - gets the last valid IPv6 address in a given IPv6 CIDR
func GetLastAddr6(cidr6 string) (*ipaddr.IPAddress, error) {
	currentCidr := ipaddr.NewIPAddressString(cidr6).GetAddress()
	if !currentCidr.IsIPv6() {
		return nil, fmt.Errorf("invalid IPv6 CIDR provided to GetLastAddr6")
	}
	upper := currentCidr.GetUpper()
	return upper, nil
}
