package ipam

import (
	"net/netip"
	"testing"
)

// mustAddr parses an IP address string and panics on failure.
func mustAddr(s string) netip.Addr {
	a, err := netip.ParseAddr(s)
	if err != nil {
		panic(err)
	}
	return a
}

// ---------------------------------------------------------------------------
// IPv4 tests
// ---------------------------------------------------------------------------

// TestIPv4AllocatorBasic verifies that the first AllocateNode returns the
// first usable IP (10.0.0.1) and the first AllocateExtclient returns the last
// usable IP (10.0.0.14) on a /28 subnet.
func TestIPv4AllocatorBasic(t *testing.T) {
	a, err := NewIPv4Allocator("10.0.0.0/28", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	n1, err := a.AllocateNode()
	if err != nil {
		t.Fatal(err)
	}
	if n1 != mustAddr("10.0.0.1") {
		t.Errorf("AllocateNode: got %v, want 10.0.0.1", n1)
	}

	e1, err := a.AllocateExtclient()
	if err != nil {
		t.Fatal(err)
	}
	if e1 != mustAddr("10.0.0.14") {
		t.Errorf("AllocateExtclient: got %v, want 10.0.0.14", e1)
	}
}

// TestIPv4AllocatorPreSeeded verifies that when the allocator is constructed
// with existing node IPs, the node cursor is anchored to the maximum of those
// IPs so the next AllocateNode call advances past all of them.
func TestIPv4AllocatorPreSeeded(t *testing.T) {
	existing := []netip.Addr{mustAddr("10.0.0.1"), mustAddr("10.0.0.3")}
	a, err := NewIPv4Allocator("10.0.0.0/28", existing, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Cursor is anchored at 10.0.0.3, so next node IP should be 10.0.0.4.
	n, err := a.AllocateNode()
	if err != nil {
		t.Fatal(err)
	}
	if n != mustAddr("10.0.0.4") {
		t.Errorf("AllocateNode after pre-seed: got %v, want 10.0.0.4", n)
	}
}

// TestIPv4AllocatorFreelist verifies the full deallocate-and-reuse cycle,
// including the territory guards and double-free protection.
func TestIPv4AllocatorFreelist(t *testing.T) {
	a, err := NewIPv4Allocator("10.0.0.0/28", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	n1, _ := a.AllocateNode()      // 10.0.0.1
	n2, _ := a.AllocateNode()      // 10.0.0.2
	e1, _ := a.AllocateExtclient() // 10.0.0.14
	e2, _ := a.AllocateExtclient() // 10.0.0.13

	// Free n1 and e1; free list should now contain [10.0.0.1, 10.0.0.14].
	if err := a.Deallocate(n1); err != nil {
		t.Fatal(err)
	}
	if err := a.Deallocate(e1); err != nil {
		t.Fatal(err)
	}

	// AllocateNode must recycle 10.0.0.1 (front of free list, ≤ nodeCursor).
	reused, err := a.AllocateNode()
	if err != nil {
		t.Fatal(err)
	}
	if reused != n1 {
		t.Errorf("expected recycled node IP %v, got %v", n1, reused)
	}

	// AllocateExtclient must recycle 10.0.0.14 (back of free list, ≥ extCursor).
	reusedExt, err := a.AllocateExtclient()
	if err != nil {
		t.Fatal(err)
	}
	if reusedExt != e1 {
		t.Errorf("expected recycled extclient IP %v, got %v", e1, reusedExt)
	}

	// Double-free: deallocate n2, then try to deallocate it again.
	if err := a.Deallocate(n2); err != nil {
		t.Fatal(err)
	}
	if err := a.Deallocate(n2); err != ErrNotAllocated {
		t.Errorf("expected ErrNotAllocated on double-free, got %v", err)
	}

	_ = e2
}

// TestIPv4FreelistOnlyNodeIPs verifies that when the free list contains only
// node-territory IPs (all ≤ nodeCursor), AllocateExtclient ignores the free
// list and advances the extclient cursor instead.
func TestIPv4FreelistOnlyNodeIPs(t *testing.T) {
	a, err := NewIPv4Allocator("10.0.0.0/28", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	n1, _ := a.AllocateNode()    // 10.0.0.1
	_, _ = a.AllocateExtclient() // 10.0.0.14 — establishes extCursor
	_ = a.Deallocate(n1)         // free list: [10.0.0.1]

	// The free list only has a node-territory IP. AllocateExtclient must not
	// take it and must instead advance the extclient cursor to 10.0.0.13.
	e2, err := a.AllocateExtclient()
	if err != nil {
		t.Fatal(err)
	}
	if e2 != mustAddr("10.0.0.13") {
		t.Errorf("AllocateExtclient: got %v, want 10.0.0.13", e2)
	}
}

// TestIPv4FreelistOnlyExtclientIPs verifies that when the free list contains
// only extclient-territory IPs (all ≥ extCursor), AllocateNode ignores the
// free list and advances the node cursor instead.
func TestIPv4FreelistOnlyExtclientIPs(t *testing.T) {
	a, err := NewIPv4Allocator("10.0.0.0/28", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = a.AllocateNode()        // 10.0.0.1 — establishes nodeCursor
	e1, _ := a.AllocateExtclient() // 10.0.0.14
	_ = a.Deallocate(e1)           // free list: [10.0.0.14]

	// The free list only has an extclient-territory IP. AllocateNode must not
	// take it and must instead advance the node cursor to 10.0.0.2.
	n2, err := a.AllocateNode()
	if err != nil {
		t.Fatal(err)
	}
	if n2 != mustAddr("10.0.0.2") {
		t.Errorf("AllocateNode: got %v, want 10.0.0.2", n2)
	}
}

// TestIPv4AllocatorNodeExhaustion verifies that a /30 subnet (2 usable IPs)
// is exhausted after exactly 2 AllocateNode calls.
func TestIPv4AllocatorNodeExhaustion(t *testing.T) {
	// 10.0.0.0/30: network=10.0.0.0, usable=10.0.0.1-10.0.0.2, broadcast=10.0.0.3
	a, err := NewIPv4Allocator("10.0.0.0/30", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// First allocation: 10.0.0.1
	n1, err := a.AllocateNode()
	if err != nil {
		t.Fatalf("first AllocateNode: unexpected error: %v", err)
	}
	if n1 != mustAddr("10.0.0.1") {
		t.Errorf("first AllocateNode: got %v, want 10.0.0.1", n1)
	}

	// Second allocation: 10.0.0.2
	n2, err := a.AllocateNode()
	if err != nil {
		t.Fatalf("second AllocateNode: unexpected error: %v", err)
	}
	if n2 != mustAddr("10.0.0.2") {
		t.Errorf("second AllocateNode: got %v, want 10.0.0.2", n2)
	}

	// Third allocation must fail — broadcast address is a sentinel.
	_, err = a.AllocateNode()
	if err != ErrExhausted {
		t.Errorf("third AllocateNode: expected ErrExhausted, got %v", err)
	}
}

// TestIPv4AllocatorExtclientExhaustion verifies that a /30 subnet (2 usable
// IPs) is exhausted after exactly 2 AllocateExtclient calls.
func TestIPv4AllocatorExtclientExhaustion(t *testing.T) {
	// 10.0.0.0/30: network=10.0.0.0, usable=10.0.0.1-10.0.0.2, broadcast=10.0.0.3
	a, err := NewIPv4Allocator("10.0.0.0/30", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// First allocation: 10.0.0.2
	e1, err := a.AllocateExtclient()
	if err != nil {
		t.Fatalf("first AllocateExtclient: unexpected error: %v", err)
	}
	if e1 != mustAddr("10.0.0.2") {
		t.Errorf("first AllocateExtclient: got %v, want 10.0.0.2", e1)
	}

	// Second allocation: 10.0.0.1
	e2, err := a.AllocateExtclient()
	if err != nil {
		t.Fatalf("second AllocateExtclient: unexpected error: %v", err)
	}
	if e2 != mustAddr("10.0.0.1") {
		t.Errorf("second AllocateExtclient: got %v, want 10.0.0.1", e2)
	}

	// Third allocation must fail — network address is a sentinel.
	_, err = a.AllocateExtclient()
	if err != ErrExhausted {
		t.Errorf("third AllocateExtclient: expected ErrExhausted, got %v", err)
	}
}

// TestIPv4InvalidCIDR verifies that NewIPv4Allocator rejects a malformed CIDR.
func TestIPv4InvalidCIDR(t *testing.T) {
	_, err := NewIPv4Allocator("not-a-cidr", nil, nil)
	if err != ErrInvalidIP {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

// TestIPv4InvalidNodeIP verifies that NewIPv4Allocator rejects a non-IPv4
// address in the nodeIPs slice.
func TestIPv4InvalidNodeIP(t *testing.T) {
	_, err := NewIPv4Allocator("10.0.0.0/28", []netip.Addr{mustAddr("fd00::1")}, nil)
	if err != ErrInvalidIP {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

// TestIPv4OutOfRangeNodeIP verifies that NewIPv4Allocator rejects a node IP
// that falls outside the subnet.
func TestIPv4OutOfRangeNodeIP(t *testing.T) {
	_, err := NewIPv4Allocator("10.0.0.0/28", []netip.Addr{mustAddr("192.168.1.1")}, nil)
	if err != ErrInvalidIP {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// IPv6 tests
// ---------------------------------------------------------------------------

// TestIPv6AllocatorBasic verifies that the first AllocateNode returns the
// first usable IP (fd00::1) and the first AllocateExtclient returns the last
// usable IP (fd00::fe) on a /120 subnet.
func TestIPv6AllocatorBasic(t *testing.T) {
	a, err := NewIPv6Allocator("fd00::/120", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	n1, err := a.AllocateNode()
	if err != nil {
		t.Fatal(err)
	}
	if n1 != mustAddr("fd00::1") {
		t.Errorf("AllocateNode: got %v, want fd00::1", n1)
	}

	e1, err := a.AllocateExtclient()
	if err != nil {
		t.Fatal(err)
	}
	if e1 != mustAddr("fd00::fe") {
		t.Errorf("AllocateExtclient: got %v, want fd00::fe", e1)
	}
}

// TestIPv6AllocatorPreSeeded verifies that when the allocator is constructed
// with existing node IPs, the node cursor is anchored to the maximum of those
// IPs so the next AllocateNode call advances past all of them.
func TestIPv6AllocatorPreSeeded(t *testing.T) {
	existing := []netip.Addr{mustAddr("fd00::1"), mustAddr("fd00::3")}
	a, err := NewIPv6Allocator("fd00::/120", existing, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Cursor is anchored at fd00::3, so next node IP should be fd00::4.
	n, err := a.AllocateNode()
	if err != nil {
		t.Fatal(err)
	}
	if n != mustAddr("fd00::4") {
		t.Errorf("AllocateNode after pre-seed: got %v, want fd00::4", n)
	}
}

// TestIPv6AllocatorFreelist verifies the full deallocate-and-reuse cycle,
// including the territory guards and double-free protection.
func TestIPv6AllocatorFreelist(t *testing.T) {
	a, err := NewIPv6Allocator("fd00::/120", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	n1, _ := a.AllocateNode()      // fd00::1
	n2, _ := a.AllocateNode()      // fd00::2
	e1, _ := a.AllocateExtclient() // fd00::fe
	e2, _ := a.AllocateExtclient() // fd00::fd

	// Free n1 and e1; free list should now contain [fd00::1, fd00::fe].
	if err := a.Deallocate(n1); err != nil {
		t.Fatal(err)
	}
	if err := a.Deallocate(e1); err != nil {
		t.Fatal(err)
	}

	// AllocateNode must recycle fd00::1 (front of free list, ≤ nodeCursor).
	reused, err := a.AllocateNode()
	if err != nil {
		t.Fatal(err)
	}
	if reused != n1 {
		t.Errorf("expected recycled node IP %v, got %v", n1, reused)
	}

	// AllocateExtclient must recycle fd00::fe (back of free list, ≥ extCursor).
	reusedExt, err := a.AllocateExtclient()
	if err != nil {
		t.Fatal(err)
	}
	if reusedExt != e1 {
		t.Errorf("expected recycled extclient IP %v, got %v", e1, reusedExt)
	}

	// Double-free: deallocate n2, then try to deallocate it again.
	if err := a.Deallocate(n2); err != nil {
		t.Fatal(err)
	}
	if err := a.Deallocate(n2); err != ErrNotAllocated {
		t.Errorf("expected ErrNotAllocated on double-free, got %v", err)
	}

	_ = e2
}

// TestIPv6FreelistOnlyNodeIPs verifies that when the free list contains only
// node-territory IPs (all ≤ nodeCursor), AllocateExtclient ignores the free
// list and advances the extclient cursor instead.
func TestIPv6FreelistOnlyNodeIPs(t *testing.T) {
	a, err := NewIPv6Allocator("fd00::/120", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	n1, _ := a.AllocateNode()    // fd00::1
	_, _ = a.AllocateExtclient() // fd00::fe — establishes extCursor
	_ = a.Deallocate(n1)         // free list: [fd00::1]

	// The free list only has a node-territory IP. AllocateExtclient must not
	// take it and must instead advance the extclient cursor to fd00::fd.
	e2, err := a.AllocateExtclient()
	if err != nil {
		t.Fatal(err)
	}
	if e2 != mustAddr("fd00::fd") {
		t.Errorf("AllocateExtclient: got %v, want fd00::fd", e2)
	}
}

// TestIPv6FreelistOnlyExtclientIPs verifies that when the free list contains
// only extclient-territory IPs (all ≥ extCursor), AllocateNode ignores the
// free list and advances the node cursor instead.
func TestIPv6FreelistOnlyExtclientIPs(t *testing.T) {
	a, err := NewIPv6Allocator("fd00::/120", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = a.AllocateNode()        // fd00::1 — establishes nodeCursor
	e1, _ := a.AllocateExtclient() // fd00::fe
	_ = a.Deallocate(e1)           // free list: [fd00::fe]

	// The free list only has an extclient-territory IP. AllocateNode must not
	// take it and must instead advance the node cursor to fd00::2.
	n2, err := a.AllocateNode()
	if err != nil {
		t.Fatal(err)
	}
	if n2 != mustAddr("fd00::2") {
		t.Errorf("AllocateNode: got %v, want fd00::2", n2)
	}
}

// TestIPv6AllocatorNodeExhaustion verifies that a /126 subnet (2 usable IPs:
// fd00::1 and fd00::2) is exhausted after exactly 2 AllocateNode calls.
// fd00::0 is networkAddr and fd00::3 is subnetMax — both are sentinels.
func TestIPv6AllocatorNodeExhaustion(t *testing.T) {
	// fd00::/126: networkAddr=fd00::0, usable=fd00::1-fd00::2, subnetMax=fd00::3
	a, err := NewIPv6Allocator("fd00::/126", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// First allocation: fd00::1
	n1, err := a.AllocateNode()
	if err != nil {
		t.Fatalf("first AllocateNode: unexpected error: %v", err)
	}
	if n1 != mustAddr("fd00::1") {
		t.Errorf("first AllocateNode: got %v, want fd00::1", n1)
	}

	// Second allocation: fd00::2
	n2, err := a.AllocateNode()
	if err != nil {
		t.Fatalf("second AllocateNode: unexpected error: %v", err)
	}
	if n2 != mustAddr("fd00::2") {
		t.Errorf("second AllocateNode: got %v, want fd00::2", n2)
	}

	// Third allocation must fail — fd00::3 is subnetMax sentinel.
	_, err = a.AllocateNode()
	if err != ErrExhausted {
		t.Errorf("third AllocateNode: expected ErrExhausted, got %v", err)
	}
}

// TestIPv6AllocatorExtclientExhaustion verifies that a /126 subnet (2 usable
// IPs: fd00::1 and fd00::2) is exhausted after exactly 2 AllocateExtclient
// calls. fd00::0 is networkAddr and fd00::3 is subnetMax — both are sentinels.
func TestIPv6AllocatorExtclientExhaustion(t *testing.T) {
	// fd00::/126: networkAddr=fd00::0, usable=fd00::1-fd00::2, subnetMax=fd00::3
	a, err := NewIPv6Allocator("fd00::/126", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// First allocation: fd00::2
	e1, err := a.AllocateExtclient()
	if err != nil {
		t.Fatalf("first AllocateExtclient: unexpected error: %v", err)
	}
	if e1 != mustAddr("fd00::2") {
		t.Errorf("first AllocateExtclient: got %v, want fd00::2", e1)
	}

	// Second allocation: fd00::1
	e2, err := a.AllocateExtclient()
	if err != nil {
		t.Fatalf("second AllocateExtclient: unexpected error: %v", err)
	}
	if e2 != mustAddr("fd00::1") {
		t.Errorf("second AllocateExtclient: got %v, want fd00::1", e2)
	}

	// Third allocation must fail — fd00::0 is networkAddr sentinel.
	_, err = a.AllocateExtclient()
	if err != ErrExhausted {
		t.Errorf("third AllocateExtclient: expected ErrExhausted, got %v", err)
	}
}

// TestIPv6InvalidCIDR verifies that NewIPv6Allocator rejects a malformed CIDR.
func TestIPv6InvalidCIDR(t *testing.T) {
	_, err := NewIPv6Allocator("not-a-cidr", nil, nil)
	if err != ErrInvalidIP {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

// TestIPv6InvalidNodeIP verifies that NewIPv6Allocator rejects a non-IPv6
// address in the nodeIPs slice.
func TestIPv6InvalidNodeIP(t *testing.T) {
	_, err := NewIPv6Allocator("fd00::/120", []netip.Addr{mustAddr("10.0.0.1")}, nil)
	if err != ErrInvalidIP {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

// TestIPv6OutOfRangeNodeIP verifies that NewIPv6Allocator rejects a node IP
// that falls outside the subnet.
func TestIPv6OutOfRangeNodeIP(t *testing.T) {
	_, err := NewIPv6Allocator("fd00::/120", []netip.Addr{mustAddr("fd01::1")}, nil)
	if err != ErrInvalidIP {
		t.Errorf("expected ErrInvalidIP, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// insertSorted tests
// ---------------------------------------------------------------------------

// TestInsertSortedIPv4 verifies that insertSorted maintains ascending order
// for IPv4 addresses and handles head, tail, and middle insertions.
func TestInsertSortedIPv4(t *testing.T) {
	var fl []netip.Addr
	fl = insertSorted(fl, mustAddr("10.0.0.3"))
	fl = insertSorted(fl, mustAddr("10.0.0.1"))
	fl = insertSorted(fl, mustAddr("10.0.0.5"))
	fl = insertSorted(fl, mustAddr("10.0.0.2"))

	want := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.5"}
	if len(fl) != len(want) {
		t.Fatalf("len: got %d, want %d", len(fl), len(want))
	}
	for i, w := range want {
		if fl[i] != mustAddr(w) {
			t.Errorf("fl[%d]: got %v, want %v", i, fl[i], w)
		}
	}
}

// TestIPv4CursorsMeetFreelistRecycle verifies that when both cursors have
// advanced until they meet (the subnet is fully allocated), freeing IPs allows
// subsequent allocation calls to recycle those addresses from the free list
// rather than returning ErrExhausted.
//
// Scenario on 10.0.0.0/28 (14 usable IPs: 10.0.0.1–10.0.0.14):
//   - Allocate all 14 IPs (nodes take .1–.7, extclients take .14–.8).
//   - Confirm the subnet is now exhausted.
//   - Free two node IPs (.1, .2) and two extclient IPs (.14, .13).
//   - Verify those exact IPs are recycled by subsequent allocation calls.
//   - Verify exhaustion resumes once the free list is empty again.
func TestIPv4CursorsMeetFreelistRecycle(t *testing.T) {
	// 10.0.0.0/28: network=10.0.0.0, usable=10.0.0.1–10.0.0.14, broadcast=10.0.0.15
	a, err := NewIPv4Allocator("10.0.0.0/28", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Allocate all 14 usable IPs: 7 nodes from the low end, 7 extclients from
	// the high end.
	var nodes, exts []netip.Addr
	for i := 0; i < 7; i++ {
		n, err := a.AllocateNode()
		if err != nil {
			t.Fatalf("AllocateNode %d: %v", i+1, err)
		}
		nodes = append(nodes, n)
	}
	for i := 0; i < 7; i++ {
		e, err := a.AllocateExtclient()
		if err != nil {
			t.Fatalf("AllocateExtclient %d: %v", i+1, err)
		}
		exts = append(exts, e)
	}

	// Both cursors have met — subnet is fully exhausted.
	if _, err := a.AllocateNode(); err != ErrExhausted {
		t.Errorf("expected ErrExhausted after full allocation, got %v", err)
	}
	if _, err := a.AllocateExtclient(); err != ErrExhausted {
		t.Errorf("expected ErrExhausted after full allocation, got %v", err)
	}

	// Free two node IPs and two extclient IPs.
	// nodes[0]=10.0.0.1, nodes[1]=10.0.0.2, exts[0]=10.0.0.14, exts[1]=10.0.0.13
	for _, ip := range []netip.Addr{nodes[0], nodes[1], exts[0], exts[1]} {
		if err := a.Deallocate(ip); err != nil {
			t.Fatalf("Deallocate(%v): %v", ip, err)
		}
	}

	// AllocateNode must recycle the freed node IPs from the front of the free
	// list — going back to the beginning of the address space.
	r1, err := a.AllocateNode()
	if err != nil {
		t.Fatalf("AllocateNode after free: %v", err)
	}
	if r1 != nodes[0] {
		t.Errorf("AllocateNode recycle 1: got %v, want %v", r1, nodes[0])
	}
	r2, err := a.AllocateNode()
	if err != nil {
		t.Fatalf("AllocateNode after free: %v", err)
	}
	if r2 != nodes[1] {
		t.Errorf("AllocateNode recycle 2: got %v, want %v", r2, nodes[1])
	}

	// AllocateExtclient must recycle the freed extclient IPs from the back of
	// the free list.
	r3, err := a.AllocateExtclient()
	if err != nil {
		t.Fatalf("AllocateExtclient after free: %v", err)
	}
	if r3 != exts[0] {
		t.Errorf("AllocateExtclient recycle 1: got %v, want %v", r3, exts[0])
	}
	r4, err := a.AllocateExtclient()
	if err != nil {
		t.Fatalf("AllocateExtclient after free: %v", err)
	}
	if r4 != exts[1] {
		t.Errorf("AllocateExtclient recycle 2: got %v, want %v", r4, exts[1])
	}

	// Free list is drained — exhausted again.
	if _, err := a.AllocateNode(); err != ErrExhausted {
		t.Errorf("expected ErrExhausted after draining free list, got %v", err)
	}
	if _, err := a.AllocateExtclient(); err != ErrExhausted {
		t.Errorf("expected ErrExhausted after draining free list, got %v", err)
	}
}

// TestIPv6CursorsMeetFreelistRecycle verifies that when both cursors have
// advanced until they meet (the subnet is fully allocated), freeing IPs allows
// subsequent allocation calls to recycle those addresses from the free list
// rather than returning ErrExhausted.
//
// Scenario on fd00::/120 (254 usable IPs: fd00::1–fd00::fe):
//   - Allocate all 254 IPs (nodes take ::1–::7f, extclients take ::fe–::80).
//   - Confirm the subnet is now exhausted.
//   - Free two node IPs (::1, ::2) and two extclient IPs (::fe, ::fd).
//   - Verify those exact IPs are recycled by subsequent allocation calls.
//   - Verify exhaustion resumes once the free list is empty again.
func TestIPv6CursorsMeetFreelistRecycle(t *testing.T) {
	// fd00::/120: networkAddr=fd00::0, usable=fd00::1–fd00::fe, subnetMax=fd00::ff
	a, err := NewIPv6Allocator("fd00::/120", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Allocate all 254 usable IPs: 127 nodes from the low end, 127 extclients
	// from the high end.
	var nodes, exts []netip.Addr
	for i := 0; i < 127; i++ {
		n, err := a.AllocateNode()
		if err != nil {
			t.Fatalf("AllocateNode %d: %v", i+1, err)
		}
		nodes = append(nodes, n)
	}
	for i := 0; i < 127; i++ {
		e, err := a.AllocateExtclient()
		if err != nil {
			t.Fatalf("AllocateExtclient %d: %v", i+1, err)
		}
		exts = append(exts, e)
	}

	// Both cursors have met — subnet is fully exhausted.
	if _, err := a.AllocateNode(); err != ErrExhausted {
		t.Errorf("expected ErrExhausted after full allocation, got %v", err)
	}
	if _, err := a.AllocateExtclient(); err != ErrExhausted {
		t.Errorf("expected ErrExhausted after full allocation, got %v", err)
	}

	// Free two node IPs and two extclient IPs.
	// nodes[0]=fd00::1, nodes[1]=fd00::2, exts[0]=fd00::fe, exts[1]=fd00::fd
	for _, ip := range []netip.Addr{nodes[0], nodes[1], exts[0], exts[1]} {
		if err := a.Deallocate(ip); err != nil {
			t.Fatalf("Deallocate(%v): %v", ip, err)
		}
	}

	// AllocateNode must recycle the freed node IPs from the front of the free
	// list — going back to the beginning of the address space.
	r1, err := a.AllocateNode()
	if err != nil {
		t.Fatalf("AllocateNode after free: %v", err)
	}
	if r1 != nodes[0] {
		t.Errorf("AllocateNode recycle 1: got %v, want %v", r1, nodes[0])
	}
	r2, err := a.AllocateNode()
	if err != nil {
		t.Fatalf("AllocateNode after free: %v", err)
	}
	if r2 != nodes[1] {
		t.Errorf("AllocateNode recycle 2: got %v, want %v", r2, nodes[1])
	}

	// AllocateExtclient must recycle the freed extclient IPs from the back of
	// the free list.
	r3, err := a.AllocateExtclient()
	if err != nil {
		t.Fatalf("AllocateExtclient after free: %v", err)
	}
	if r3 != exts[0] {
		t.Errorf("AllocateExtclient recycle 1: got %v, want %v", r3, exts[0])
	}
	r4, err := a.AllocateExtclient()
	if err != nil {
		t.Fatalf("AllocateExtclient after free: %v", err)
	}
	if r4 != exts[1] {
		t.Errorf("AllocateExtclient recycle 2: got %v, want %v", r4, exts[1])
	}

	// Free list is drained — exhausted again.
	if _, err := a.AllocateNode(); err != ErrExhausted {
		t.Errorf("expected ErrExhausted after draining free list, got %v", err)
	}
	if _, err := a.AllocateExtclient(); err != ErrExhausted {
		t.Errorf("expected ErrExhausted after draining free list, got %v", err)
	}
}

// TestInsertSortedIPv6 verifies that insertSorted maintains ascending order
// for IPv6 addresses and handles head, tail, and middle insertions.
func TestInsertSortedIPv6(t *testing.T) {
	var fl []netip.Addr
	fl = insertSorted(fl, mustAddr("fd00::3"))
	fl = insertSorted(fl, mustAddr("fd00::1"))
	fl = insertSorted(fl, mustAddr("fd00::5"))
	fl = insertSorted(fl, mustAddr("fd00::2"))

	want := []string{"fd00::1", "fd00::2", "fd00::3", "fd00::5"}
	if len(fl) != len(want) {
		t.Fatalf("len: got %d, want %d", len(fl), len(want))
	}
	for i, w := range want {
		if fl[i] != mustAddr(w) {
			t.Errorf("fl[%d]: got %v, want %v", i, fl[i], w)
		}
	}
}
