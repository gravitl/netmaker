package logic

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
)

func ValidateEgressReq(e *schema.Egress) error {
	if e.Network == "" {
		return errors.New("network id is empty")
	}
	if e.Nat && (e.Mode != models.DirectNAT && e.Mode != models.VirtualNAT) {
		return errors.New("invalid NAT type")
	}
	if !e.Nat {
		e.Mode = ""
		e.VirtualRange = ""
	}
	if e.Domain != "" && e.Nat {
		e.Mode = models.DirectNAT
	}
	_, err := logic.GetNetwork(e.Network)
	if err != nil {
		return errors.New("failed to get network " + err.Error())
	}

	if !servercfg.IsPro && len(e.Nodes) > 1 {
		return errors.New("can only set one routing node on CE")
	}

	if len(e.Nodes) > 0 {
		for k := range e.Nodes {
			_, err := logic.GetNodeByID(k)
			if err != nil {
				return errors.New("invalid routing node " + err.Error())
			}
		}
	}
	if len(e.Tags) > 0 {
		e.Nodes = make(datatypes.JSONMap)
		for tagID := range e.Tags {
			_, err := GetTag(models.TagID(tagID))
			if err != nil {
				return errors.New("invalid tag " + tagID)
			}
		}
	}
	return nil
}

func RemoveTagFromEgress(net models.NetworkID, tagID models.TagID) {
	eli, _ := (&schema.Egress{Network: net.String()}).ListByNetwork(db.WithContext(context.TODO()))
	for _, eI := range eli {
		if _, ok := eI.Tags[tagID.String()]; ok {
			delete(eI.Tags, tagID.String())
			eI.Update(db.WithContext(context.TODO()))
		}
	}
}

func AssignVirtualRangeToEgress(nw *models.Network, eg *schema.Egress) error {
	if nw == nil {
		return fmt.Errorf("network is nil")
	}
	if eg == nil {
		return fmt.Errorf("egress is nil")
	}
	if !eg.Nat {
		logger.Log(2, "AssignVirtualRangeToEgress: NAT not enabled, skipping virtual range assignment")
		return nil
	}

	// v1: only allocate for virtual NAT mode
	if eg.Mode != models.VirtualNAT {
		logger.Log(2, "AssignVirtualRangeToEgress: mode is not VirtualNAT, skipping. Mode:", string(eg.Mode))
		return nil
	}

	// already assigned
	if eg.VirtualRange != "" {
		logger.Log(2, "AssignVirtualRangeToEgress: virtual range already assigned:", eg.VirtualRange)
		return nil
	}

	// Determine if we need IPv4 or IPv6 based on the egress range
	isIPv6 := false
	egressPrefixLen := 0

	if eg.Range != "" && eg.Range != "*" {
		_, ipNet, err := net.ParseCIDR(eg.Range)
		if err == nil && ipNet != nil {
			isIPv6 = ipNet.IP.To4() == nil
			_, egressPrefixLen = ipNet.Mask.Size()
			logger.Log(2, fmt.Sprintf("AssignVirtualRangeToEgress: parsed egress range '%s' as IPv%d with prefix length /%d", eg.Range, map[bool]int{false: 4, true: 6}[isIPv6], egressPrefixLen))
		} else {
			logger.Log(2, fmt.Sprintf("AssignVirtualRangeToEgress: failed to parse egress range '%s' as CIDR: %v", eg.Range, err))
			// Try parsing as IP address
			ip := net.ParseIP(eg.Range)
			if ip != nil {
				isIPv6 = ip.To4() == nil
				if isIPv6 {
					egressPrefixLen = 128 // Default to /128 for single IPv6 address
				} else {
					egressPrefixLen = 32 // Default to /32 for single IPv4 address
				}
				logger.Log(2, fmt.Sprintf("AssignVirtualRangeToEgress: parsed egress range '%s' as IP address (IPv%d), defaulting to /%d", eg.Range, map[bool]int{false: 4, true: 6}[isIPv6], egressPrefixLen))
			}
		}
	} else if len(eg.DomainAns) > 0 {
		// If Range is empty or "*", check DomainAns for IP addresses
		for _, domainAns := range eg.DomainAns {
			_, ipNet, err := net.ParseCIDR(domainAns)
			if err == nil && ipNet != nil {
				if ipNet.IP.To4() == nil {
					isIPv6 = true
					_, egressPrefixLen = ipNet.Mask.Size()
					logger.Log(2, fmt.Sprintf("AssignVirtualRangeToEgress: found IPv6 address in DomainAns '%s' with prefix length /%d", domainAns, egressPrefixLen))
					break
				} else if egressPrefixLen == 0 {
					// Use the first IPv4 prefix length found
					_, egressPrefixLen = ipNet.Mask.Size()
				}
			}
		}
	}

	var poolCIDR string
	var maxSitePrefixLen int
	var poolName string

	if isIPv6 {
		if nw.VirtualNATPoolIPv6 == "" || nw.VirtualNATSitePrefixLenIPv6 == 0 {
			return fmt.Errorf("virtual NAT IPv6 pool not configured for network %s", nw.NetID)
		}
		poolCIDR = nw.VirtualNATPoolIPv6
		maxSitePrefixLen = nw.VirtualNATSitePrefixLenIPv6
		poolName = "IPv6"
	} else {
		if nw.VirtualNATPoolIPv4 == "" || nw.VirtualNATSitePrefixLenIPv4 == 0 {
			return fmt.Errorf("virtual NAT IPv4 pool not configured for network %s", nw.NetID)
		}
		poolCIDR = nw.VirtualNATPoolIPv4
		maxSitePrefixLen = nw.VirtualNATSitePrefixLenIPv4
		poolName = "IPv4"
	}

	// Determine the prefix length to use for allocation
	// For IPv4: match the egress range size to avoid wasting address space
	// For IPv6: always use the configured site prefix length (typically /64) as IPv6 has abundant address space
	sitePrefixLen := maxSitePrefixLen
	if !isIPv6 && egressPrefixLen > 0 {
		// For IPv4, try to match the egress range size
		_, poolNet, err := net.ParseCIDR(poolCIDR)
		if err == nil && poolNet != nil {
			poolPrefixLen, bits := poolNet.Mask.Size()
			// Use egress prefix length if it's valid (>= pool prefix, <= address space)
			// Allow more specific (larger prefix length) than configured max to match egress range exactly
			if egressPrefixLen >= poolPrefixLen && egressPrefixLen <= bits {
				sitePrefixLen = egressPrefixLen
				if egressPrefixLen > maxSitePrefixLen {
					logger.Log(1, fmt.Sprintf("AssignVirtualRangeToEgress: IPv4 - matching egress range size /%d (more specific than configured max /%d)", sitePrefixLen, maxSitePrefixLen))
				} else {
					logger.Log(1, fmt.Sprintf("AssignVirtualRangeToEgress: IPv4 - matching egress range size /%d (configured max is /%d)", sitePrefixLen, maxSitePrefixLen))
				}
			} else {
				logger.Log(1, fmt.Sprintf("AssignVirtualRangeToEgress: IPv4 - egress range /%d is invalid for pool (pool prefix: /%d, address space: /%d), using configured max /%d", egressPrefixLen, poolPrefixLen, bits, sitePrefixLen))
			}
		}
	} else if isIPv6 {
		logger.Log(1, fmt.Sprintf("AssignVirtualRangeToEgress: IPv6 - using configured site prefix length /%d", sitePrefixLen))
	}

	logger.Log(1, fmt.Sprintf("AssignVirtualRangeToEgress: allocating virtual %s range for egress %s network %s pool %s prefixLen %d", poolName, eg.ID, eg.Network, poolCIDR, sitePrefixLen))

	// load already allocated virtual ranges in this network (read-only)
	var allocated []string
	if err := db.FromContext(db.WithContext(context.TODO())).Model(&schema.Egress{}).
		Where("network = ? AND virtual_range IS NOT NULL AND virtual_range <> ''", eg.Network).
		Pluck("virtual_range", &allocated).Error; err != nil {
		logger.Log(0, "AssignVirtualRangeToEgress: error querying allocated ranges:", err.Error())
		return err
	}

	// Filter out any invalid/empty values and filter by IP version
	validAllocated := make([]string, 0, len(allocated))
	for _, a := range allocated {
		if a != "" && a != "<nil>" {
			_, ipNet, err := net.ParseCIDR(strings.TrimSpace(a))
			if err == nil && ipNet != nil {
				// Only include ranges of the same IP version
				allocatedIsIPv6 := ipNet.IP.To4() == nil
				if allocatedIsIPv6 == isIPv6 {
					validAllocated = append(validAllocated, a)
				}
			}
		}
	}
	allocated = validAllocated

	logger.Log(1, fmt.Sprintf("AssignVirtualRangeToEgress: found %d already allocated %s ranges: %v", len(allocated), poolName, allocated))

	// allocate a free prefix from the pool and set on model (no DB write here)
	virtualCIDR, err := allocateNextPrefixDeterministic(
		poolCIDR,
		sitePrefixLen,
		allocated,
		eg.ID,
	)
	if err != nil {
		logger.Log(0, "AssignVirtualRangeToEgress: error allocating prefix:", err.Error())
		return err
	}

	if virtualCIDR == "" {
		logger.Log(0, fmt.Sprintf("AssignVirtualRangeToEgress: allocateNextPrefixDeterministic returned empty string without error for egress %s", eg.ID))
		return fmt.Errorf("failed to allocate virtual range: function returned empty string")
	}

	logger.Log(1, fmt.Sprintf("AssignVirtualRangeToEgress: allocated virtual range '%s' for egress %s", virtualCIDR, eg.ID))

	// Validate that the allocated virtual range matches the IP version of the egress range
	_, allocatedNet, err := net.ParseCIDR(virtualCIDR)
	if err == nil && allocatedNet != nil {
		allocatedIsIPv6 := allocatedNet.IP.To4() == nil
		if allocatedIsIPv6 != isIPv6 {
			return fmt.Errorf("IP version mismatch: allocated virtual range '%s' is IPv%d but egress range requires IPv%d", virtualCIDR, map[bool]int{false: 4, true: 6}[allocatedIsIPv6], map[bool]int{false: 4, true: 6}[isIPv6])
		}
	}

	eg.VirtualRange = virtualCIDR
	return nil
}

func allocateNextPrefixDeterministic(poolCIDR string, sitePrefixLen int, allocated []string, seed string) (string, error) {
	_, pool, err := net.ParseCIDR(strings.TrimSpace(poolCIDR))
	if err != nil {
		return "", fmt.Errorf("invalid pool cidr %q: %w", poolCIDR, err)
	}

	poolPrefixLen, bits := pool.Mask.Size()
	if sitePrefixLen < poolPrefixLen || sitePrefixLen > bits {
		return "", fmt.Errorf("sitePrefixLen %d invalid for pool %s", sitePrefixLen, poolCIDR)
	}

	allocSet := map[string]struct{}{}
	for _, a := range allocated {
		if a == "" {
			continue // Skip empty strings
		}
		_, an, e := net.ParseCIDR(strings.TrimSpace(a))
		if e == nil && an != nil {
			allocSet[an.String()] = struct{}{}
		}
	}

	total := 1 << uint(sitePrefixLen-poolPrefixLen)
	start := hashIndex(seed, total)

	logger.Log(2, fmt.Sprintf("allocateNextPrefixDeterministic: pool=%s poolPrefixLen=%d sitePrefixLen=%d total=%d start=%d seed=%s allocated=%v allocSet size=%d", poolCIDR, poolPrefixLen, sitePrefixLen, total, start, seed, allocated, len(allocSet)))

	checked := 0
	nilCount := 0
	invalidCount := 0
	usedCount := 0
	for i := 0; i < total; i++ {
		idx := (start + i) % total
		cand := nthSubnet(pool, sitePrefixLen, idx)
		if cand == nil {
			nilCount++
			if nilCount <= 5 { // Log first 5 nil cases
				logger.Log(2, fmt.Sprintf("allocateNextPrefixDeterministic: nthSubnet returned nil for idx=%d", idx))
			}
			continue
		}
		cs := cand.String()
		if cs == "" || cs == "<nil>" {
			invalidCount++
			if invalidCount <= 5 { // Log first 5 invalid cases
				logger.Log(2, fmt.Sprintf("allocateNextPrefixDeterministic: nthSubnet returned invalid IPNet at idx=%d (String()='%s')", idx, cs))
			}
			continue
		}
		checked++
		if _, used := allocSet[cs]; !used {
			logger.Log(1, fmt.Sprintf("allocateNextPrefixDeterministic: found free prefix %s at idx=%d (checked %d, nil: %d, invalid: %d, used: %d)", cs, idx, checked, nilCount, invalidCount, usedCount))
			return cs, nil
		}
		usedCount++
		if usedCount <= 5 { // Log first 5 used cases
			logger.Log(2, fmt.Sprintf("allocateNextPrefixDeterministic: prefix %s at idx=%d is already used", cs, idx))
		}
	}

	logger.Log(0, fmt.Sprintf("allocateNextPrefixDeterministic: exhausted all %d possibilities (checked: %d valid, nil: %d, invalid: %d, used: %d)", total, checked, nilCount, invalidCount, usedCount))

	return "", fmt.Errorf("no available prefixes left in pool %s for /%d", poolCIDR, sitePrefixLen)
}

func nthSubnet(pool *net.IPNet, newPrefixLen int, n int) *net.IPNet {
	if pool == nil {
		return nil
	}

	poolPrefixLen, bits := pool.Mask.Size()
	if newPrefixLen < poolPrefixLen || newPrefixLen > bits || n < 0 {
		return nil
	}

	// base IP as big.Int
	base := ipToBigInt(pool.IP)

	// subnet size = 2^(bits - newPrefixLen)
	size := new(big.Int).Lsh(big.NewInt(1), uint(bits-newPrefixLen))

	// offset = n * size
	offset := new(big.Int).Mul(big.NewInt(int64(n)), size)

	ipInt := new(big.Int).Add(base, offset)
	ip := bigIntToIP(ipInt, bits)
	mask := net.CIDRMask(newPrefixLen, bits)
	return &net.IPNet{IP: ip.Mask(mask), Mask: mask}
}

func ipToBigInt(ip net.IP) *big.Int {
	ip = ip.To16()
	if ip == nil {
		return big.NewInt(0)
	}
	return new(big.Int).SetBytes(ip)
}

func bigIntToIP(i *big.Int, bits int) net.IP {
	b := i.Bytes()
	byteLen := bits / 8
	// Pad on the left to ensure we have exactly byteLen bytes
	for len(b) < byteLen {
		b = append([]byte{0}, b...)
	}
	// Take only the last byteLen bytes in case we have more
	if len(b) > byteLen {
		b = b[len(b)-byteLen:]
	}
	ip := net.IP(b)
	if bits == 32 {
		return ip.To4()
	}
	return ip
}

func hashIndex(siteID string, mod int) int {
	if mod <= 1 {
		return 0
	}
	sum := sha1.Sum([]byte(siteID))
	// Use first 4 bytes as uint32
	v := binary.BigEndian.Uint32(sum[:4])
	return int(v % uint32(mod))
}
