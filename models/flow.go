package models

import (
	"fmt"
	"strings"
	"time"

	nfct "github.com/ti-mo/conntrack"
)

type FlowEventType int

const (
	FlowStart   FlowEventType = iota
	FlowUpdate  FlowEventType = iota
	FlowDestroy FlowEventType = iota
)

func (f FlowEventType) String() string {
	switch f {
	case FlowStart:
		return "Start"
	case FlowUpdate:
		return "Update"
	case FlowDestroy:
		return "Destroy"
	default:
		return fmt.Sprintf("Unknown(%d)", f)
	}
}

type FlowEvent struct {
	ID            uint32           `json:"id"`
	Type          FlowEventType    `json:"type"`
	Status        nfct.Status      `json:"status"`
	ProtocolInfo  FlowProtocolInfo `json:"protocol_info"`
	OriginPeer    FlowPeer         `json:"origin_peer"`
	ReplyPeer     FlowPeer         `json:"reply_peer"`
	OriginCounter FlowCounter      `json:"origin_counter"`
	ReplyCounter  FlowCounter      `json:"reply_counter"`
	Timestamp     FlowTimestamp    `json:"timestamp"`
}

func (f *FlowEvent) String() string {
	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "FlowEvent #%d\n", f.ID)
	_, _ = fmt.Fprintf(&b, "  Type: %s\n", f.Type.String())
	_, _ = fmt.Fprintf(&b, "  Status: %s\n", f.Status.String())
	_, _ = fmt.Fprintf(&b, "  Protocol: %s\n", f.ProtocolInfo.String())

	// Peers
	_, _ = fmt.Fprintf(&b, "  Origin Peer: %s\n", f.OriginPeer.String())
	_, _ = fmt.Fprintf(&b, "  Reply Peer: %s\n", f.ReplyPeer.String())

	// Counters
	_, _ = fmt.Fprintf(&b, "  Origin Counter: %s\n", f.OriginCounter.String())
	_, _ = fmt.Fprintf(&b, "  Reply Counter: %s\n", f.ReplyCounter.String())

	_, _ = fmt.Fprintf(&b, "  Timestamp: %s\n", f.Timestamp.String())

	return b.String()
}

type FlowProtocolInfo struct {
	Protocol uint8 `json:"protocol"`

	// ICMP Enrichment
	ICMPType uint8 `json:"icmp_type"`
	ICMPCode uint8 `json:"icmp_code"`

	// TCP Enrichment
	TCPState               uint8  `json:"tcp_state"`
	TCPOriginalWindowScale uint8  `json:"tcp_original_window_scale"`
	TCPReplyWindowScale    uint8  `json:"tcp_reply_window_scale"`
	TCPOriginalFlags       uint16 `json:"tcp_original_flags"`
	TCPReplyFlags          uint16 `json:"tcp_reply_flags"`
}

func (p *FlowProtocolInfo) String() string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%d", p.Protocol)

	if p.Protocol == 6 {
		_, _ = fmt.Fprintf(
			&b,
			" [TCP] [state: %d origin ws: %d reply ws: %d origin flags: 0x%x reply flags: 0x%x]",
			p.TCPState,
			p.TCPOriginalWindowScale, p.TCPReplyWindowScale,
			p.TCPOriginalFlags, p.TCPReplyFlags,
		)
	} else if p.Protocol == 1 || p.Protocol == 58 {
		_, _ = fmt.Fprintf(&b, " [ICMP] [type=%d code=%d]", p.ICMPType, p.ICMPCode)
	} else if p.Protocol == 17 {
		_, _ = fmt.Fprintf(&b, " [UDP]")
	}

	return b.String()
}

type FlowPeer struct {
	IP   string `json:"ip"`
	Port uint16 `json:"port"`
}

func (p *FlowPeer) String() string {
	if p.Port == 0 {
		return p.IP
	}

	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

type FlowCounter struct {
	Packets uint64 `json:"packets"`
	Bytes   uint64 `json:"bytes"`
}

func (c *FlowCounter) String() string {
	return fmt.Sprintf("packets=%d bytes=%d", c.Packets, c.Bytes)
}

type FlowTimestamp struct {
	Start     time.Time `json:"start"`
	Stop      time.Time `json:"stop"`
	EventTime time.Time `json:"event_time"`
}

func (t *FlowTimestamp) String() string {
	var b strings.Builder
	var prefix string

	if !t.Start.IsZero() {
		_, _ = fmt.Fprintf(&b, "%sstart: %s", prefix, t.Start.Format(time.RFC3339))
		prefix = " "
	}

	if !t.Stop.IsZero() {
		_, _ = fmt.Fprintf(&b, "%sstop: %s", prefix, t.Stop.Format(time.RFC3339))
		prefix = " "
	}

	if !t.EventTime.IsZero() {
		_, _ = fmt.Fprintf(&b, "%sevent_time: %s", prefix, t.EventTime.Format(time.RFC3339))
	}

	return b.String()
}
