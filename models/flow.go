package models

import (
	"fmt"
	"net/netip"
	"strings"
	"time"

	nfct "github.com/ti-mo/conntrack"
)

type FlowEventType int

const (
	FlowStart   FlowEventType = iota
	FlowDestroy FlowEventType = iota
)

func (f FlowEventType) String() string {
	switch f {
	case FlowStart:
		return "Start"
	case FlowDestroy:
		return "Destroy"
	default:
		return fmt.Sprintf("Unknown(%d)", f)
	}
}

type FlowEvent struct {
	ID            uint32        `json:"id"`
	Type          FlowEventType `json:"type"`
	Status        nfct.Status   `json:"status"`
	Protocol      uint8         `json:"protocol"`
	ICMPType      uint8         `json:"icmp_type"`
	ICMPCode      uint8         `json:"icmp_code"`
	OriginIP      netip.Addr    `json:"origin_ip"`
	OriginPort    uint16        `json:"origin_port"`
	ReplyIP       netip.Addr    `json:"reply_ip"`
	ReplyPort     uint16        `json:"reply_port"`
	OriginPackets uint64        `json:"origin_packets"`
	OriginBytes   uint64        `json:"origin_bytes"`
	ReplyPackets  uint64        `json:"reply_packets"`
	ReplyBytes    uint64        `json:"reply_bytes"`
	EventTime     time.Time     `json:"event_time"`
	FlowStart     time.Time     `json:"flow_start"`
	FlowDestroy   time.Time     `json:"flow_destroy"`
}

func (f *FlowEvent) String() string {
	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "FlowEvent #%d\n", f.ID)
	_, _ = fmt.Fprintf(&b, "  Type: %s\n", f.Type.String())
	_, _ = fmt.Fprintf(&b, "  Status: %s\n", f.Status.String())
	_, _ = fmt.Fprintf(&b, "  Protocol: %d\n", f.Protocol)

	// Peers
	_, _ = fmt.Fprintf(&b, "  Origin Peer: %s:%d\n", f.OriginIP.String(), f.OriginPort)
	_, _ = fmt.Fprintf(&b, "  Reply Peer: %s:%d\n", f.ReplyIP.String(), f.ReplyPort)

	// Counters
	_, _ = fmt.Fprintf(&b, "  Origin Counter: %d %d\n", f.OriginPackets, f.OriginBytes)
	_, _ = fmt.Fprintf(&b, "  Reply Counter: %d %d\n", f.ReplyPackets, f.ReplyBytes)

	_, _ = fmt.Fprintf(&b, "  Timestamp: %s %s %s\n", f.EventTime.String(), f.FlowStart.String(), f.FlowDestroy.String())

	return b.String()
}
