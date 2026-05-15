package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
)

// ── IPv4 Public ──────────────────────────────────────────────────────────────

// privateIPv4Blocks holds all RFC-reserved IPv4 ranges that are NOT public.
var privateIPv4Blocks []*net.IPNet

func init() {
	reserved := []string{
		"0.0.0.0/8",          // "This" network
		"10.0.0.0/8",         // RFC 1918 private
		"100.64.0.0/10",      // Shared address space (RFC 6598)
		"127.0.0.0/8",        // Loopback
		"169.254.0.0/16",     // Link-local
		"172.16.0.0/12",      // RFC 1918 private
		"192.0.0.0/24",       // IETF protocol assignments
		"192.0.2.0/24",       // TEST-NET-1 (documentation)
		"192.88.99.0/24",     // IPv6-to-IPv4 relay (deprecated)
		"192.168.0.0/16",     // RFC 1918 private
		"198.18.0.0/15",      // Benchmarking
		"198.51.100.0/24",    // TEST-NET-2 (documentation)
		"203.0.113.0/24",     // TEST-NET-3 (documentation)
		"224.0.0.0/4",        // Multicast
		"240.0.0.0/4",        // Reserved / future use
		"255.255.255.255/32", // Broadcast
	}
	for _, cidr := range reserved {
		_, block, _ := net.ParseCIDR(cidr)
		privateIPv4Blocks = append(privateIPv4Blocks, block)
	}
}

func isPrivateIPv4(ip net.IP) bool {
	ip = ip.To4()
	if ip == nil {
		return true
	}
	for _, block := range privateIPv4Blocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// RandomPublicIPv4 returns a random, globally-routable IPv4 address.
// It rejects any address that falls in a reserved/private range.
func RandomPublicIPv4() (net.IP, error) {
	for {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("random read: %w", err)
		}
		ip := net.IP(b)
		if !isPrivateIPv4(ip) {
			return ip, nil
		}
	}
}

// ── IPv6 Public ──────────────────────────────────────────────────────────────

var privateIPv6Blocks []*net.IPNet

func init() {
	reserved6 := []string{
		"::/128",        // Unspecified
		"::1/128",       // Loopback
		"::ffff:0:0/96", // IPv4-mapped
		"64:ff9b::/96",  // IPv4/IPv6 translation
		"100::/64",      // Discard
		"2001::/32",     // Teredo
		"2001:db8::/32", // Documentation
		"2002::/16",     // 6to4
		"fc00::/7",      // Unique local (fc00::/7 covers fc00:: and fd00::)
		"fe80::/10",     // Link-local
		"ff00::/8",      // Multicast
	}
	for _, cidr := range reserved6 {
		_, block, _ := net.ParseCIDR(cidr)
		privateIPv6Blocks = append(privateIPv6Blocks, block)
	}
}

func isPrivateIPv6(ip net.IP) bool {
	ip = ip.To16()
	if ip == nil {
		return true
	}
	for _, block := range privateIPv6Blocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// RandomPublicIPv6 returns a random, globally-routable IPv6 address.
// It rejects any address that falls in a reserved/private range.
func RandomPublicIPv6() (net.IP, error) {
	for {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("random read: %w", err)
		}
		ip := net.IP(b)
		if !isPrivateIPv6(ip) {
			return ip, nil
		}
	}
}

// ── Private / Shared CIDR generation ────────────────────────────────────────

// RandomPrivateCIDRv4 returns a random prefix within the RFC 6598
// shared address space 100.64.0.0/10, with given prefix length.
func RandomPrivateCIDRv4(prefixLen int) (*net.IPNet, error) {
	// 100.64.0.0/10 base = 0x64400000
	base := uint32(0x64400000)
	// Host bits available inside /10: 32-10 = 22 bits.
	hostBits := 32 - prefixLen
	availableHostBits := 32 - 10 // 22

	// How many extra bits can we randomise (above the chosen prefix)?
	randomBits := availableHostBits - (prefixLen - 10)
	if randomBits < 0 {
		randomBits = 0
	}

	var offset uint32
	if randomBits > 0 {
		n, err := randomIntInRange(0, (1<<uint(randomBits))-1)
		if err != nil {
			return nil, err
		}
		offset = uint32(n) << uint(hostBits)
	}

	addr := make([]byte, 4)
	binary.BigEndian.PutUint32(addr, base|offset)

	return &net.IPNet{
		IP:   addr,
		Mask: net.CIDRMask(prefixLen, 32),
	}, nil
}

// RandomPrivateCIDRv6 returns a random prefix within the RFC 4193
// unique-local range fd00::/8 (the locally-assigned half of fc00::/7),
// with given prefix length.
func RandomPrivateCIDRv6(prefixLen int) (*net.IPNet, error) {
	// Generate a fully random 128-bit address, then force the top byte to 0xfd
	// (fd00::/8 — ULA with L=1).
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("random read: %w", err)
	}
	b[0] = 0xfd

	// Mask to the chosen prefix so the network address is canonical.
	mask := net.CIDRMask(prefixLen, 128)
	for i := range b {
		b[i] &= mask[i]
	}

	return &net.IPNet{
		IP:   b,
		Mask: mask,
	}, nil
}

// ── Random IP within a CIDR ──────────────────────────────────────────────────

// RandomIPInCIDR returns a random host address within the given network.
// It works for both IPv4 and IPv6 CIDRs.
//
// For IPv4 /31 and /32 (and IPv6 /127, /128) all addresses are returned
// as-is without filtering network/broadcast addresses, which is consistent
// with modern point-to-point link behaviour (RFC 3021).
func RandomIPInCIDR(network *net.IPNet) (net.IP, error) {
	// Normalize to the right length.
	ip := network.IP
	ones, bits := network.Mask.Size()
	if bits == 0 {
		return nil, fmt.Errorf("invalid mask")
	}

	size := bits / 8 // 4 or 16
	base := make([]byte, size)

	// Use To4 / To16 to normalize.
	if bits == 32 {
		v4 := ip.To4()
		if v4 == nil {
			return nil, fmt.Errorf("expected IPv4 address")
		}
		copy(base, v4)
	} else {
		v6 := ip.To16()
		if v6 == nil {
			return nil, fmt.Errorf("expected IPv6 address")
		}
		copy(base, v6)
	}

	hostBits := bits - ones
	if hostBits == 0 {
		// /32 or /128 — only one address.
		return base, nil
	}

	// Compute the number of host addresses: 2^hostBits.
	count := new(big.Int).Lsh(big.NewInt(1), uint(hostBits))

	// Pick a random offset in [0, count).
	offset, err := rand.Int(rand.Reader, count)
	if err != nil {
		return nil, fmt.Errorf("random int: %w", err)
	}

	// Add offset to the base address (big-endian byte arithmetic).
	baseInt := new(big.Int).SetBytes(base)
	result := new(big.Int).Add(baseInt, offset)

	resultBytes := result.Bytes()

	// Pad to the correct length (big.Int strips leading zeros).
	addr := make([]byte, size)
	copy(addr[size-len(resultBytes):], resultBytes)

	return addr, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// randomIntInRange returns a cryptographically random int64 in [min, max].
func randomIntInRange(min, max int64) (int, error) {
	if min == max {
		return int(min), nil
	}
	span := big.NewInt(max - min + 1)
	n, err := rand.Int(rand.Reader, span)
	if err != nil {
		return 0, err
	}
	return int(n.Int64() + min), nil
}
