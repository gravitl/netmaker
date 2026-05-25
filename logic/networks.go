package logic

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

// DeleteNetwork - deletes a network
func DeleteNetwork(network string, force bool, done chan struct{}) error {
	defer func() {
		// Delete default network enrollment key
		keys, _ := GetAllEnrollmentKeys()
		for _, key := range keys {
			if key.Default && len(key.Tags) > 0 && key.Tags[0] == network {
				_ = DeleteEnrollmentKey(key.Value, true)
				break
			}
		}

		_ = DeleteNetworkDNS(network)
	}()

	nodeCount, err := GetNetworkNonServerNodeCount(network)
	if nodeCount == 0 || database.IsEmptyRecord(err) {
		_network := &schema.Network{
			Name: network,
		}
		// delete server nodes first then db records
		return _network.Delete(db.WithContext(context.TODO()))
	}

	// Remove All Nodes
	go func() {
		nodes, err := GetNetworkNodes(network)
		if err == nil {
			for _, node := range nodes {
				node := node
				host := &schema.Host{ID: node.HostID}
				if err := host.Get(db.WithContext(context.TODO())); err != nil {
					continue
				}
				if node.IsGw {
					// delete ext clients belonging to gateway
					DeleteGatewayExtClients(node.ID.String(), node.Network)
				}
				DissasociateNodeFromHost(&node, host)
			}
		}
		// delete server nodes first then db records
		_network := &schema.Network{
			Name: network,
		}
		err = _network.Delete(db.WithContext(context.TODO()))
		if err != nil {
			return
		}
		done <- struct{}{}
		close(done)
	}()

	return nil
}

// AssignVirtualNATDefaults determines safe defaults based on VPN CIDR
func AssignVirtualNATDefaults(network *schema.Network, vpnCIDR string) {
	const (
		cgnatCIDR        = "100.64.0.0/10"
		fallbackIPv4Pool = "198.18.0.0/15"

		defaultIPv4SitePrefix = 24
	)

	// Parse CGNAT CIDR (should always succeed, but check for safety)
	_, cgnatNet, err := net.ParseCIDR(cgnatCIDR)
	if err != nil {
		// Fallback to default pool if CGNAT parsing fails (shouldn't happen)
		network.VirtualNATPoolIPv4 = fallbackIPv4Pool
		network.VirtualNATSitePrefixLenIPv4 = defaultIPv4SitePrefix
		return
	}

	var virtualIPv4Pool string
	// Parse VPN CIDR - if it fails or is empty, use fallback
	if vpnCIDR == "" {
		virtualIPv4Pool = fallbackIPv4Pool
	} else {
		_, vpnNet, err := net.ParseCIDR(vpnCIDR)
		if err != nil || vpnNet == nil {
			// Invalid VPN CIDR, use fallback
			virtualIPv4Pool = fallbackIPv4Pool
		} else if !cidrOverlaps(vpnNet, cgnatNet) {
			// Safe to reuse VPN CIDR for Virtual NAT
			virtualIPv4Pool = vpnCIDR
		} else {
			// VPN is CGNAT — must not reuse
			virtualIPv4Pool = fallbackIPv4Pool
		}
	}

	network.VirtualNATPoolIPv4 = virtualIPv4Pool
	network.VirtualNATSitePrefixLenIPv4 = defaultIPv4SitePrefix
}

// cidrOverlaps checks if two CIDR blocks overlap
func cidrOverlaps(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

const (
	FallbackVNATPool    = "198.18.0.0/15"
	VNATPoolPrefixLen   = 22
	DefaultSitePrefixV4 = 24
	CgnatCIDR           = "100.64.0.0/10"
)

// AllocateUniqueVNATPool allocates a unique Virtual NAT pool for a network,
// ensuring it doesn't conflict with pools already assigned to other networks.
func AllocateUniqueVNATPool(network *schema.Network) error {
	networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	allocatedPools := make(map[string]struct{})
	for _, n := range networks {
		if n.VirtualNATSitePrefixLenIPv4 > 0 {
			if _, _, err := net.ParseCIDR(n.VirtualNATPoolIPv4); err == nil {
				allocatedPools[n.VirtualNATPoolIPv4] = struct{}{}
			}
		}
	}

	_, cgnatNet, err := net.ParseCIDR(CgnatCIDR)
	if err != nil {
		return fmt.Errorf("failed to parse CGNAT CIDR: %w", err)
	}

	_, fallbackNet, err := net.ParseCIDR(FallbackVNATPool)
	if err != nil {
		return fmt.Errorf("failed to parse fallback pool: %w", err)
	}

	vpnCIDR := network.AddressRange
	needsUniquePool := false

	if vpnCIDR == "" {
		needsUniquePool = true
	} else {
		_, vpnNet, err := net.ParseCIDR(vpnCIDR)
		if err != nil || vpnNet == nil {
			needsUniquePool = true
		} else if cidrOverlaps(vpnNet, cgnatNet) {
			needsUniquePool = true
		}
	}

	if needsUniquePool {
		uniquePool := AllocateUniquePoolFromFallback(fallbackNet, VNATPoolPrefixLen, allocatedPools, network.Name)
		if uniquePool == "" {
			return fmt.Errorf("failed to allocate unique Virtual NAT pool for network %s: pool exhausted", network.Name)
		}
		network.VirtualNATPoolIPv4 = uniquePool
		network.VirtualNATSitePrefixLenIPv4 = DefaultSitePrefixV4
	} else {
		AssignVirtualNATDefaults(network, vpnCIDR)
	}

	return nil
}

// AllocateUniquePoolFromFallback allocates a unique subnet of the given prefix length
// from the fallback pool, skipping any subnets already present in the allocated map.
func AllocateUniquePoolFromFallback(pool *net.IPNet, newPrefixLen int, allocated map[string]struct{}, seed string) string {
	if pool == nil {
		return ""
	}

	poolPrefixLen, bits := pool.Mask.Size()
	if newPrefixLen < poolPrefixLen || newPrefixLen > bits {
		return ""
	}

	total := 1 << uint(newPrefixLen-poolPrefixLen)
	start := vnatHashIndex(seed, total)

	for i := 0; i < total; i++ {
		idx := (start + i) % total
		cand := NthSubnet(pool, newPrefixLen, idx)
		if cand == nil || cand.IP == nil {
			continue
		}
		cs := cand.String()
		if _, _, err := net.ParseCIDR(cs); err != nil {
			continue
		}
		if _, used := allocated[cs]; !used {
			return cs
		}
	}

	return ""
}

// NthSubnet calculates the nth subnet of a given prefix length within a pool.
func NthSubnet(pool *net.IPNet, newPrefixLen int, n int) *net.IPNet {
	if pool == nil {
		return nil
	}

	poolPrefixLen, bits := pool.Mask.Size()
	if newPrefixLen < poolPrefixLen || newPrefixLen > bits || n < 0 {
		return nil
	}

	base := ipToBigInt(pool.IP)
	size := new(big.Int).Lsh(big.NewInt(1), uint(bits-newPrefixLen))
	offset := new(big.Int).Mul(big.NewInt(int64(n)), size)
	ipInt := new(big.Int).Add(base, offset)
	ip := bigIntToIP(ipInt, bits)

	mask := net.CIDRMask(newPrefixLen, bits)
	return &net.IPNet{IP: ip.Mask(mask), Mask: mask}
}

func ipToBigInt(ip net.IP) *big.Int {
	if v4 := ip.To4(); v4 != nil {
		return new(big.Int).SetBytes(v4)
	}
	if v6 := ip.To16(); v6 != nil {
		return new(big.Int).SetBytes(v6)
	}
	return big.NewInt(0)
}

func bigIntToIP(i *big.Int, bits int) net.IP {
	b := i.Bytes()
	byteLen := bits / 8
	if len(b) < byteLen {
		pad := make([]byte, byteLen-len(b))
		b = append(pad, b...)
	}
	ip := net.IP(b)
	if bits == 32 {
		return ip.To4()
	}
	return ip
}

func vnatHashIndex(seed string, mod int) int {
	if mod <= 1 {
		return 0
	}
	sum := sha1.Sum([]byte(seed))
	v := binary.BigEndian.Uint32(sum[:4])
	return int(v % uint32(mod))
}

// CreateNetwork - creates a network in database
func CreateNetwork(_network *schema.Network) error {
	if _network.AddressRange != "" {
		normalizedRange, err := NormalizeCIDR(_network.AddressRange)
		if err != nil {
			return err
		}
		_network.AddressRange = normalizedRange
	}
	if _network.AddressRange6 != "" {
		normalizedRange, err := NormalizeCIDR(_network.AddressRange6)
		if err != nil {
			return err
		}
		_network.AddressRange6 = normalizedRange
	}
	if !IsNetworkCIDRUnique(GetNetworkNetworkCIDR4(_network), GetNetworkNetworkCIDR6(_network)) {
		return errors.New("network cidr already in use")
	}

	_network.NodesUpdatedAt = time.Now().UTC()

	err := ValidateNetwork(_network, false)
	if err != nil {
		//logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return err
	}

	err = _network.Create(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	_, _ = CreateEnrollmentKey(
		0,
		time.Time{},
		[]string{_network.Name},
		[]string{_network.Name},
		[]models.TagID{},
		true,
		uuid.Nil,
		true,
		false,
		false,
	)

	return nil
}

func GetNetworkNetworkCIDR4(network *schema.Network) *net.IPNet {
	if network.AddressRange == "" {
		return nil
	}
	_, netCidr, _ := net.ParseCIDR(network.AddressRange)
	return netCidr
}
func GetNetworkNetworkCIDR6(network *schema.Network) *net.IPNet {
	if network.AddressRange6 == "" {
		return nil
	}
	_, netCidr, _ := net.ParseCIDR(network.AddressRange6)
	return netCidr
}

// GetNetworkNonServerNodeCount - get number of network non server nodes
func GetNetworkNonServerNodeCount(networkName string) (int, error) {
	nodes, err := GetNetworkNodes(networkName)
	return len(nodes), err
}

func IsNetworkCIDRUnique(cidr4 *net.IPNet, cidr6 *net.IPNet) bool {
	networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return errors.Is(err, gorm.ErrRecordNotFound)
	}
	for _, network := range networks {
		if intersect(GetNetworkNetworkCIDR4(&network), cidr4) ||
			intersect(GetNetworkNetworkCIDR6(&network), cidr6) {
			return false
		}
	}
	return true
}

func intersect(n1, n2 *net.IPNet) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

// IsNetworkNameUnique - checks to see if any other networks have the same name (id)
func IsNetworkNameUnique(network *schema.Network) (bool, error) {
	_network := &schema.Network{
		Name: network.Name,
	}
	err := _network.Get(db.WithContext(context.TODO()))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}

		return false, err
	}

	return false, nil
}

func UpsertNetwork(_network *schema.Network) error {
	return _network.Update(db.WithContext(context.TODO()))
}

// UpdateNetwork - updates a network with another network's fields
func UpdateNetwork(currentNetwork, newNetwork *schema.Network) error {
	if err := ValidateNetwork(newNetwork, true); err != nil {
		return err
	}
	if newNetwork.Name != currentNetwork.Name {
		return errors.New("failed to update network " + newNetwork.Name + ", cannot change netid.")
	}
	featureFlags := GetFeatureFlags()
	if featureFlags.EnableDeviceApproval {
		currentNetwork.AutoJoin = newNetwork.AutoJoin
	} else {
		currentNetwork.AutoJoin = true
	}
	currentNetwork.AutoRemove = newNetwork.AutoRemove
	currentNetwork.AutoRemoveThreshold = newNetwork.AutoRemoveThreshold
	currentNetwork.AutoRemoveTags = newNetwork.AutoRemoveTags

	// Validate and update Virtual NAT IPv4 settings
	if newNetwork.VirtualNATPoolIPv4 != "" {
		_, poolNet, err := net.ParseCIDR(newNetwork.VirtualNATPoolIPv4)
		if err != nil {
			return fmt.Errorf("invalid Virtual NAT IPv4 pool CIDR: %w", err)
		}
		poolPrefixLen, _ := poolNet.Mask.Size()

		if newNetwork.VirtualNATSitePrefixLenIPv4 <= 0 || newNetwork.VirtualNATSitePrefixLenIPv4 > 32 {
			return fmt.Errorf("invalid Virtual NAT IPv4 site prefix length: must be between 1 and 32, got %d", newNetwork.VirtualNATSitePrefixLenIPv4)
		}
		// Validate that site prefix length is not larger (less specific) than pool prefix length
		// e.g., pool /24 and site /8 is invalid because /8 is less specific (larger CIDR) than /24
		// Site prefix must be >= pool prefix (more specific or equal)
		if newNetwork.VirtualNATSitePrefixLenIPv4 < poolPrefixLen {
			return fmt.Errorf("invalid Virtual NAT IPv4 site prefix length: site prefix length /%d cannot be larger (less specific) than pool prefix length /%d. Site prefix must be >= pool prefix (more specific or equal)", newNetwork.VirtualNATSitePrefixLenIPv4, poolPrefixLen)
		}
		currentNetwork.VirtualNATPoolIPv4 = newNetwork.VirtualNATPoolIPv4
		currentNetwork.VirtualNATSitePrefixLenIPv4 = newNetwork.VirtualNATSitePrefixLenIPv4
	} else if newNetwork.VirtualNATSitePrefixLenIPv4 > 0 {
		// If pool is empty but site prefix is provided, validate against existing pool
		if currentNetwork.VirtualNATPoolIPv4 != "" {
			_, poolNet, err := net.ParseCIDR(currentNetwork.VirtualNATPoolIPv4)
			if err == nil {
				poolPrefixLen, _ := poolNet.Mask.Size()
				if newNetwork.VirtualNATSitePrefixLenIPv4 > 32 {
					return fmt.Errorf("invalid Virtual NAT IPv4 site prefix length: must be between 1 and 32, got %d", newNetwork.VirtualNATSitePrefixLenIPv4)
				}
				// Validate that site prefix length is not larger (less specific) than pool prefix length
				if newNetwork.VirtualNATSitePrefixLenIPv4 < poolPrefixLen {
					return fmt.Errorf("invalid Virtual NAT IPv4 site prefix length: site prefix length /%d cannot be larger (less specific) than pool prefix length /%d. Site prefix must be >= pool prefix (more specific or equal)", newNetwork.VirtualNATSitePrefixLenIPv4, poolPrefixLen)
				}
			}
		}
		currentNetwork.VirtualNATSitePrefixLenIPv4 = newNetwork.VirtualNATSitePrefixLenIPv4
	}
	// When both VNAT fields are omitted from the update, preserve existing settings
	return currentNetwork.Update(db.WithContext(context.TODO()))
}

// validateNetName - checks if a netid of a network uses valid characters
func validateNetName(network *schema.Network) error {
	var validationErr error

	if len(network.Name) == 0 {
		validationErr = errors.Join(validationErr, errors.New("network name cannot be empty"))
	}

	if len(network.Name) > 32 {
		validationErr = errors.Join(validationErr, errors.New("network name cannot be longer than 32 characters"))
	}

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_"
	for _, char := range network.Name {
		if !strings.Contains(charset, string(char)) {
			validationErr = errors.Join(validationErr, errors.New("invalid character(s) in network name"))
			break
		}
	}

	return validationErr
}

// Validate - validates fields of an network struct
func ValidateNetwork(network *schema.Network, isUpdate bool) error {
	var validationErr error
	err := validateNetName(network)
	if err != nil {
		validationErr = errors.Join(validationErr, err)
	}

	if !isUpdate {
		nameUnique, _ := IsNetworkNameUnique(network)
		if !nameUnique {
			validationErr = errors.Join(validationErr, errors.New("invalid network name"))
		}
	}

	if network.AddressRange != "" {
		_, _, err = net.ParseCIDR(network.AddressRange)
		if err != nil {
			validationErr = errors.Join(validationErr, err)
		}
	}

	if network.AddressRange6 != "" {
		_, _, err = net.ParseCIDR(network.AddressRange6)
		if err != nil {
			validationErr = errors.Join(validationErr, err)
		}
	}

	if network.DefaultKeepAlive > 1000 {
		validationErr = errors.Join(validationErr, errors.New("default keep alive must be less than 1000"))
	}

	return validationErr
}

// SaveNetwork - save network struct to database
func SaveNetwork(_network *schema.Network) error {
	_existingNetwork := schema.Network{Name: _network.Name}
	// Check if network exists to preserve ID
	err := _existingNetwork.Get(db.WithContext(context.TODO()))
	if err == nil {
		_network.ID = _existingNetwork.ID
		return _network.Update(db.WithContext(context.TODO()))
	}

	return _network.Create(db.WithContext(context.TODO()))
}

// NetworkExists - check if network exists
func NetworkExists(name string) (bool, error) {
	err := (&schema.Network{Name: name}).Get(db.WithContext(context.TODO()))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// SortNetworks - Sorts slice of Networks by their NetID alphabetically with numbers first
func SortNetworks(unsortedNetworks []schema.Network) {
	sort.Slice(unsortedNetworks, func(i, j int) bool {
		return unsortedNetworks[i].Name < unsortedNetworks[j].Name
	})
}

var NetworkHook models.HookFunc = func(params ...interface{}) error {
	networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	allNodes, err := GetAllNodes()
	if err != nil {
		return err
	}
	for _, network := range networks {
		if !network.AutoRemove || network.AutoRemoveThreshold == 0 {
			continue
		}
		nodes := GetNetworkNodesMemory(allNodes, network.Name)
		for _, node := range nodes {
			if !node.Connected {
				continue
			}
			exists := false
			for _, tagI := range network.AutoRemoveTags {
				if tagI == "*" {
					exists = true
					break
				}
				if _, ok := node.Tags[models.TagID(tagI)]; ok {
					exists = true
					break
				}
			}
			if !exists {
				continue
			}
			if time.Since(node.LastCheckIn) > time.Duration(network.AutoRemoveThreshold)*time.Minute {
				if err := DeleteNode(&node, true); err != nil {
					continue
				}
				node.PendingDelete = true
				node.Action = schema.NODE_DELETE
				DeleteNodesCh <- &node
				host := &schema.Host{ID: node.HostID}
				if err := host.Get(db.WithContext(context.TODO())); err == nil && len(host.Nodes) == 0 {
					(&schema.Host{ID: host.ID}).Delete(db.WithContext(context.TODO()))
				}
			}
		}
	}
	return nil
}

func InitNetworkHooks() {
	HookManagerCh <- models.HookDetails{
		ID:       "network-hook",
		Hook:     NetworkHook,
		Interval: time.Duration(GetServerSettings().CleanUpInterval) * time.Minute,
	}
}
