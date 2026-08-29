package ipam

import (
	"errors"
	"net"
	"net/netip"
	"sort"
	"sync"

	"github.com/c-robinson/iplib"
)

// Sentinel errors returned by allocator methods.
var (
	// ErrExhausted is returned when no IPs remain to allocate in the subnet.
	ErrExhausted = errors.New("ipam: no IPs remain to allocate")

	// ErrNotAllocated is returned when attempting to deallocate an IP that
	// was never allocated (or has already been returned to the free list).
	ErrNotAllocated = errors.New("ipam: IP not in allocated set")

	// ErrInvalidIP is returned when a provided IP address is malformed or
	// falls outside the allocator's subnet.
	ErrInvalidIP = errors.New("ipam: malformed or out-of-range IP")
)

// IPv4Allocator manages stateful IPv4 address allocation for two device types:
// nodes (allocated from the low end of the subnet) and extclients (allocated
// from the high end). It maintains a single shared sorted free list that both
// device types draw from, subject to territory guards that prevent either type
// from poaching the other's address range.
//
// The network address and broadcast address of the subnet are boundary
// sentinels and are never handed out.
//
// All methods are safe for concurrent use.
type IPv4Allocator struct {
	mu sync.Mutex

	// subnet is the parsed IPv4 network from which addresses are drawn.
	subnet iplib.Net4

	// allocated tracks every IP currently in use (not yet deallocated).
	allocated map[netip.Addr]struct{}

	// nodeCursor is the highest IP address that has been assigned to a node.
	// It is initialized to the network address (one below the first usable IP)
	// so that the first AllocateNode call advances to FirstAddress.
	nodeCursor netip.Addr

	// extCursor is the lowest IP address that has been assigned to an
	// extclient. It is initialized to the broadcast address (one above the
	// last usable IP) so that the first AllocateExtclient call retreats to
	// LastAddress.
	extCursor netip.Addr

	// freelist holds deallocated IPs sorted in ascending order. Nodes consume
	// from the front (smallest), extclients from the back (largest), subject
	// to territory guards.
	freelist []netip.Addr
}

// NewIPv4Allocator constructs an IPv4Allocator for the given CIDR string.
//
// nodeIPs is the set of IPs already allocated to nodes; extIPs is the set
// already allocated to extclients. Both slices may be nil or empty. The
// cursors are anchored to the maximum node IP and minimum extclient IP
// respectively, so subsequent allocations resume from where the previous
// session left off without re-issuing any address.
//
// Returns ErrInvalidIP if cidr cannot be parsed, or if any supplied IP is
// malformed or outside the subnet.
func NewIPv4Allocator(cidr string, nodeIPs, extIPs []netip.Addr) (*IPv4Allocator, error) {
	n := iplib.Net4FromStr(cidr)
	if n.IP() == nil {
		return nil, ErrInvalidIP
	}

	allocated := make(map[netip.Addr]struct{})

	// Validate and register all pre-existing node IPs.
	var nodeAddrs []netip.Addr
	for _, ip := range nodeIPs {
		if !ip.Is4() {
			return nil, ErrInvalidIP
		}
		raw := addrToNetIPv4(ip)
		if !n.Contains(raw) {
			return nil, ErrInvalidIP
		}
		allocated[ip] = struct{}{}
		nodeAddrs = append(nodeAddrs, ip)
	}

	// Validate and register all pre-existing extclient IPs.
	var extAddrs []netip.Addr
	for _, ip := range extIPs {
		if !ip.Is4() {
			return nil, ErrInvalidIP
		}
		raw := addrToNetIPv4(ip)
		if !n.Contains(raw) {
			return nil, ErrInvalidIP
		}
		allocated[ip] = struct{}{}
		extAddrs = append(extAddrs, ip)
	}

	// Node cursor starts at the network address (boundary sentinel, one below
	// FirstAddress) when no nodes exist, or at the highest known node IP so
	// that the next allocation advances past all previously issued addresses.
	nodeCursor := netIPv4ToAddr(n.NetworkAddress())
	if len(nodeAddrs) > 0 {
		nodeCursor = maxAddr(nodeAddrs)
	}

	// Ext cursor starts at the broadcast address (boundary sentinel, one above
	// LastAddress) when no extclients exist, or at the lowest known extclient
	// IP so that the next allocation retreats past all previously issued ones.
	extCursor := netIPv4ToAddr(n.BroadcastAddress())
	if len(extAddrs) > 0 {
		extCursor = minAddr(extAddrs)
	}

	return &IPv4Allocator{
		subnet:     n,
		allocated:  allocated,
		nodeCursor: nodeCursor,
		extCursor:  extCursor,
	}, nil
}

// AllocateNode picks the next available IP from the low end of the subnet and
// marks it as allocated. It first attempts to reuse a freed IP from the front
// of the free list, but only if that IP lies within the node territory (i.e.,
// it is not beyond the current node cursor). If no suitable free IP exists,
// it advances the node cursor toward the extclient territory.
//
// Returns ErrExhausted when no usable IPs remain.
func (a *IPv4Allocator) AllocateNode() (netip.Addr, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Recycle from the free list if the smallest freed IP is still within node
	// territory (i.e., not greater than the node cursor).
	if len(a.freelist) > 0 && !a.nodeCursor.Less(a.freelist[0]) {
		ip := a.freelist[0]
		a.freelist = a.freelist[1:]
		a.allocated[ip] = struct{}{}
		return ip, nil
	}

	// Advance the node cursor one step at a time, skipping already-allocated
	// IPs, until a free address is found or the subnet boundary is reached.
	cursor := addrToNetIPv4(a.nodeCursor)
	for {
		next, err := a.subnet.NextIP(cursor)
		// ErrAddressOutOfRange: cursor has passed the last IP in the subnet.
		// ErrBroadcastAddress: the next address is the broadcast — stop here.
		if errors.Is(err, iplib.ErrAddressOutOfRange) || errors.Is(err, iplib.ErrBroadcastAddress) {
			return netip.Addr{}, ErrExhausted
		}
		cursor = next
		addr := netIPv4ToAddr(cursor)
		if _, taken := a.allocated[addr]; !taken {
			a.nodeCursor = addr
			a.allocated[addr] = struct{}{}
			return addr, nil
		}
	}
}

// AllocateExtclient picks the next available IP from the high end of the
// subnet and marks it as allocated. It first attempts to reuse a freed IP
// from the back of the free list, but only if that IP lies within the
// extclient territory (i.e., it is not below the current extclient cursor).
// If no suitable free IP exists, it retreats the extclient cursor toward the
// node territory.
//
// Returns ErrExhausted when no usable IPs remain.
func (a *IPv4Allocator) AllocateExtclient() (netip.Addr, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Recycle from the free list if the largest freed IP is still within
	// extclient territory (i.e., not less than the extclient cursor).
	if len(a.freelist) > 0 && !a.freelist[len(a.freelist)-1].Less(a.extCursor) {
		ip := a.freelist[len(a.freelist)-1]
		a.freelist = a.freelist[:len(a.freelist)-1]
		a.allocated[ip] = struct{}{}
		return ip, nil
	}

	// Retreat the extclient cursor one step at a time, skipping
	// already-allocated IPs, until a free address is found or the subnet
	// boundary is reached.
	cursor := addrToNetIPv4(a.extCursor)
	for {
		prev, err := a.subnet.PreviousIP(cursor)
		// ErrAddressOutOfRange: cursor has passed the first IP in the subnet.
		// ErrNetworkAddress: the previous address is the network — stop here.
		if errors.Is(err, iplib.ErrAddressOutOfRange) || errors.Is(err, iplib.ErrNetworkAddress) {
			return netip.Addr{}, ErrExhausted
		}
		cursor = prev
		addr := netIPv4ToAddr(cursor)
		if _, taken := a.allocated[addr]; !taken {
			a.extCursor = addr
			a.allocated[addr] = struct{}{}
			return addr, nil
		}
	}
}

// Deallocate returns ip to the free list so it may be re-issued by a future
// allocation call. The free list is kept sorted in ascending order.
//
// Returns ErrNotAllocated if ip is not currently in the allocated set.
// Returns ErrInvalidIP if ip is the zero value.
func (a *IPv4Allocator) Deallocate(ip netip.Addr) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !ip.IsValid() {
		return ErrInvalidIP
	}
	if _, ok := a.allocated[ip]; !ok {
		return ErrNotAllocated
	}
	delete(a.allocated, ip)
	a.freelist = insertSorted(a.freelist, ip)
	return nil
}

// IPv6Allocator manages stateful IPv6 address allocation for two device types:
// nodes (allocated from the low end of the subnet) and extclients (allocated
// from the high end). It maintains a single shared sorted free list that both
// device types draw from, subject to territory guards that prevent either type
// from poaching the other's address range.
//
// The network address (first IP of the subnet) and the subnet maximum
// (network | ~mask, analogous to the IPv4 broadcast) are boundary sentinels
// and are never handed out.
//
// All methods are safe for concurrent use.
type IPv6Allocator struct {
	mu sync.Mutex

	// subnet is the parsed IPv6 network from which addresses are drawn.
	subnet iplib.Net6

	// allocated tracks every IP currently in use (not yet deallocated).
	allocated map[netip.Addr]struct{}

	// nodeCursor is the highest IP address that has been assigned to a node.
	// It is initialized to networkAddr (one below the first usable IP) so
	// that the first AllocateNode call advances to FirstAddress.
	nodeCursor netip.Addr

	// extCursor is the lowest IP address that has been assigned to an
	// extclient. It is initialized to subnetMax (one above the last usable IP)
	// so that the first AllocateExtclient call retreats to the last usable IP.
	extCursor netip.Addr

	// freelist holds deallocated IPs sorted in ascending order. Nodes consume
	// from the front (smallest), extclients from the back (largest), subject
	// to territory guards.
	freelist []netip.Addr

	// networkAddr is the network address of the subnet (n.IP()). It is the
	// lower boundary sentinel: never allocated, and used to detect when
	// AllocateExtclient has exhausted the usable range.
	networkAddr netip.Addr

	// subnetMax is the highest address in the subnet (network | ~mask). It is
	// the upper boundary sentinel: never allocated, and used to detect when
	// AllocateNode has exhausted the usable range.
	subnetMax netip.Addr
}

// NewIPv6Allocator constructs an IPv6Allocator for the given CIDR string.
//
// nodeIPs is the set of IPs already allocated to nodes; extIPs is the set
// already allocated to extclients. Both slices may be nil or empty. The
// cursors are anchored to the maximum node IP and minimum extclient IP
// respectively, so subsequent allocations resume from where the previous
// session left off without re-issuing any address.
//
// Returns ErrInvalidIP if cidr cannot be parsed, or if any supplied IP is
// malformed or outside the subnet.
func NewIPv6Allocator(cidr string, nodeIPs, extIPs []netip.Addr) (*IPv6Allocator, error) {
	n := iplib.Net6FromStr(cidr)
	if n.IP() == nil {
		return nil, ErrInvalidIP
	}

	allocated := make(map[netip.Addr]struct{})

	// Validate and register all pre-existing node IPs.
	var nodeAddrs []netip.Addr
	for _, ip := range nodeIPs {
		if !ip.Is6() || ip.Is4In6() {
			return nil, ErrInvalidIP
		}
		raw := addrToNetIPv6(ip)
		if !n.Contains(raw) {
			return nil, ErrInvalidIP
		}
		allocated[ip] = struct{}{}
		nodeAddrs = append(nodeAddrs, ip)
	}

	// Validate and register all pre-existing extclient IPs.
	var extAddrs []netip.Addr
	for _, ip := range extIPs {
		if !ip.Is6() || ip.Is4In6() {
			return nil, ErrInvalidIP
		}
		raw := addrToNetIPv6(ip)
		if !n.Contains(raw) {
			return nil, ErrInvalidIP
		}
		allocated[ip] = struct{}{}
		extAddrs = append(extAddrs, ip)
	}

	// networkAddr is the lower boundary sentinel (never allocated).
	networkAddr := netIPv6ToAddr(n.IP())

	// subnetMax is the upper boundary sentinel: network | ~mask (never allocated).
	subnetMax := netIPv6ToAddr(net6MaxIP(n))

	// Node cursor starts at networkAddr when no nodes exist, or at the highest
	// known node IP so that the next allocation advances past all previously
	// issued addresses.
	nodeCursor := networkAddr
	if len(nodeAddrs) > 0 {
		nodeCursor = maxAddr(nodeAddrs)
	}

	// Ext cursor starts at subnetMax when no extclients exist, or at the
	// lowest known extclient IP so that the next allocation retreats past all
	// previously issued ones.
	extCursor := subnetMax
	if len(extAddrs) > 0 {
		extCursor = minAddr(extAddrs)
	}

	return &IPv6Allocator{
		subnet:      n,
		allocated:   allocated,
		nodeCursor:  nodeCursor,
		extCursor:   extCursor,
		networkAddr: networkAddr,
		subnetMax:   subnetMax,
	}, nil
}

// AllocateNode picks the next available IP from the low end of the subnet and
// marks it as allocated. It first attempts to reuse a freed IP from the front
// of the free list, but only if that IP lies within the node territory (i.e.,
// it is not beyond the current node cursor). If no suitable free IP exists,
// it advances the node cursor toward the extclient territory.
//
// The network address and subnetMax sentinel are never returned.
//
// Returns ErrExhausted when no usable IPs remain.
func (a *IPv6Allocator) AllocateNode() (netip.Addr, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Recycle from the free list if the smallest freed IP is still within node
	// territory (i.e., not greater than the node cursor).
	if len(a.freelist) > 0 && !a.nodeCursor.Less(a.freelist[0]) {
		ip := a.freelist[0]
		a.freelist = a.freelist[1:]
		a.allocated[ip] = struct{}{}
		return ip, nil
	}

	// Advance the node cursor one step at a time, skipping already-allocated
	// IPs, until a free address is found or the subnet boundary is reached.
	cursor := addrToNetIPv6(a.nodeCursor)
	for {
		next, err := a.subnet.NextIP(cursor)
		// ErrAddressOutOfRange: cursor has passed the last IP in the subnet.
		if errors.Is(err, iplib.ErrAddressOutOfRange) {
			return netip.Addr{}, ErrExhausted
		}
		cursor = next
		addr := netIPv6ToAddr(cursor)
		// subnetMax is the upper boundary sentinel — never allocate it.
		if addr == a.subnetMax {
			return netip.Addr{}, ErrExhausted
		}
		if _, taken := a.allocated[addr]; !taken {
			a.nodeCursor = addr
			a.allocated[addr] = struct{}{}
			return addr, nil
		}
	}
}

// AllocateExtclient picks the next available IP from the high end of the
// subnet and marks it as allocated. It first attempts to reuse a freed IP
// from the back of the free list, but only if that IP lies within the
// extclient territory (i.e., it is not below the current extclient cursor).
// If no suitable free IP exists, it retreats the extclient cursor toward the
// node territory.
//
// The networkAddr and subnetMax sentinels are never returned.
//
// Returns ErrExhausted when no usable IPs remain.
func (a *IPv6Allocator) AllocateExtclient() (netip.Addr, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Recycle from the free list if the largest freed IP is still within
	// extclient territory (i.e., not less than the extclient cursor).
	if len(a.freelist) > 0 && !a.freelist[len(a.freelist)-1].Less(a.extCursor) {
		ip := a.freelist[len(a.freelist)-1]
		a.freelist = a.freelist[:len(a.freelist)-1]
		a.allocated[ip] = struct{}{}
		return ip, nil
	}

	// Retreat the extclient cursor one step at a time, skipping
	// already-allocated IPs, until a free address is found or the subnet
	// boundary is reached.
	cursor := addrToNetIPv6(a.extCursor)
	for {
		prev, err := a.subnet.PreviousIP(cursor)
		// ErrAddressOutOfRange: cursor has passed the first IP in the subnet.
		if errors.Is(err, iplib.ErrAddressOutOfRange) {
			return netip.Addr{}, ErrExhausted
		}
		cursor = prev
		addr := netIPv6ToAddr(cursor)
		// networkAddr is the lower boundary sentinel — never allocate it.
		if addr == a.networkAddr {
			return netip.Addr{}, ErrExhausted
		}
		if _, taken := a.allocated[addr]; !taken {
			a.extCursor = addr
			a.allocated[addr] = struct{}{}
			return addr, nil
		}
	}
}

// Deallocate returns ip to the free list so it may be re-issued by a future
// allocation call. The free list is kept sorted in ascending order.
//
// Returns ErrNotAllocated if ip is not currently in the allocated set.
// Returns ErrInvalidIP if ip is the zero value.
func (a *IPv6Allocator) Deallocate(ip netip.Addr) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !ip.IsValid() {
		return ErrInvalidIP
	}
	if _, ok := a.allocated[ip]; !ok {
		return ErrNotAllocated
	}
	delete(a.allocated, ip)
	a.freelist = insertSorted(a.freelist, ip)
	return nil
}

// net6MaxIP computes the highest IP address in an IPv6 subnet, analogous to
// the IPv4 broadcast address. It is calculated as network | ~mask.
func net6MaxIP(n iplib.Net6) net.IP {
	last := make(net.IP, 16)
	mask := n.Mask()
	base := n.IP()
	for i := range last {
		last[i] = base[i] | ^mask[i]
	}
	return last
}

// insertSorted inserts addr into freelist while maintaining ascending sort
// order. It uses binary search for O(log n) positioning.
func insertSorted(freelist []netip.Addr, addr netip.Addr) []netip.Addr {
	i := sort.Search(len(freelist), func(i int) bool {
		return !freelist[i].Less(addr)
	})
	freelist = append(freelist, netip.Addr{})
	copy(freelist[i+1:], freelist[i:])
	freelist[i] = addr
	return freelist
}

// maxAddr returns the largest address in addrs. It panics if addrs is empty.
func maxAddr(addrs []netip.Addr) netip.Addr {
	m := addrs[0]
	for _, a := range addrs[1:] {
		if m.Less(a) {
			m = a
		}
	}
	return m
}

// minAddr returns the smallest address in addrs. It panics if addrs is empty.
func minAddr(addrs []netip.Addr) netip.Addr {
	m := addrs[0]
	for _, a := range addrs[1:] {
		if a.Less(m) {
			m = a
		}
	}
	return m
}

// addrToNetIPv4 converts a netip.Addr (IPv4) to a 4-byte net.IP slice
// suitable for iplib IPv4 operations.
func addrToNetIPv4(addr netip.Addr) net.IP {
	a := addr.As4()
	return a[:]
}

// addrToNetIPv6 converts a netip.Addr (IPv6) to a 16-byte net.IP slice
// suitable for iplib IPv6 operations.
func addrToNetIPv6(addr netip.Addr) net.IP {
	a := addr.As16()
	return a[:]
}

// netIPv4ToAddr converts a net.IP (IPv4) to a netip.Addr. The Unmap call
// ensures the result is a true IPv4 address rather than an IPv4-in-IPv6 form.
func netIPv4ToAddr(ip net.IP) netip.Addr {
	addr, _ := netip.AddrFromSlice(ip)
	return addr.Unmap()
}

// netIPv6ToAddr converts a net.IP (IPv6, 16-byte) to a netip.Addr.
func netIPv6ToAddr(ip net.IP) netip.Addr {
	addr, _ := netip.AddrFromSlice(ip)
	return addr
}
