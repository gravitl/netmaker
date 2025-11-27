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
	ID            uint32        `json:"id"`
	Type          FlowEventType `json:"type"`
	Status        nfct.Status   `json:"status"`
	Protocol      uint8         `json:"protocol"`
	OriginPeer    Peer          `json:"origin_peer"`
	ReplyPeer     Peer          `json:"reply_peer"`
	OriginCounter Counter       `json:"origin_counter"`
	ReplyCounter  Counter       `json:"reply_counter"`
	Timestamp     time.Time     `json:"timestamp"`
	Start         time.Time     `json:"start"`
	Stop          time.Time     `json:"stop"`

	// Enriched Info
	EnrichedProtocol   *EnrichedProtocol `json:"enriched_protocol,omitempty"`
	EnrichedOriginPeer *EnrichedPeer     `json:"enriched_origin_peer,omitempty"`
	EnrichedReplyPeer  *EnrichedPeer     `json:"enriched_reply_peer,omitempty"`
}

func (f *FlowEvent) String() string {
	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "FlowEvent #%d\n", f.ID)
	_, _ = fmt.Fprintf(&b, "  Type: %s\n", f.Type.String())
	_, _ = fmt.Fprintf(&b, "  Status: %s\n", f.Status.String())

	_, _ = fmt.Fprintf(&b, "  Protocol: %d\n", f.Protocol)

	// Peers
	_, _ = fmt.Fprintf(&b, "  Origin Peer: %s\n", f.OriginPeer.String())
	_, _ = fmt.Fprintf(&b, "  Reply  Peer: %s\n", f.ReplyPeer.String())

	// Counters
	_, _ = fmt.Fprintf(&b, "  Origin Counter: %s\n", f.OriginCounter.String())
	_, _ = fmt.Fprintf(&b, "  Reply  Counter: %s\n", f.ReplyCounter.String())

	// Enriched Protocol
	if f.EnrichedProtocol != nil {
		_, _ = fmt.Fprintf(&b, "  Enriched Protocol:\n")
		ep := f.EnrichedProtocol
		if f.Protocol == 6 {
			_, _ = fmt.Fprintf(&b, "    TCP:\n")
			_, _ = fmt.Fprintf(&b, "      State: %d\n", ep.TCPState)
			_, _ = fmt.Fprintf(&b, "      Orig WS: %d  Reply WS: %d\n", ep.TCPOriginalWindowScale, ep.TCPReplyWindowScale)
			_, _ = fmt.Fprintf(&b, "      Orig Flags: 0x%x  Reply Flags: 0x%x\n", ep.TCPOriginalFlags, ep.TCPReplyFlags)
		} else {
			_, _ = fmt.Fprintf(&b, "    ICMP: type=%d code=%d\n", ep.ICMPType, ep.ICMPCode)
		}
	}

	// Enriched Peers
	if f.EnrichedOriginPeer != nil {
		_, _ = fmt.Fprintf(&b, "  Enriched Origin Peer:\n")
		_, _ = fmt.Fprintf(&b, "    NodeID: %s\n", f.EnrichedOriginPeer.NodeID)
	}

	if f.EnrichedReplyPeer != nil {
		_, _ = fmt.Fprintf(&b, "  Enriched Reply Peer:\n")
		_, _ = fmt.Fprintf(&b, "    NodeID: %s\n", f.EnrichedReplyPeer.NodeID)
	}

	if !f.Start.IsZero() {
		_, _ = fmt.Fprintf(&b, "  Start Time: %s\n", f.Start.Format(time.RFC3339))
	}

	if !f.Stop.IsZero() {
		_, _ = fmt.Fprintf(&b, "  Stop Time: %s\n", f.Stop.Format(time.RFC3339))
	}

	return b.String()
}

type Peer struct {
	IP   string `json:"ip"`
	Port uint16 `json:"port"`
}

func (p *Peer) String() string {
	if p.Port == 0 {
		return p.IP
	}

	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

type Counter struct {
	Packets uint64 `json:"packets"`
	Bytes   uint64 `json:"bytes"`
}

func (c *Counter) String() string {
	return fmt.Sprintf("packets=%d bytes=%d", c.Packets, c.Bytes)
}

type EnrichedProtocol struct {
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

type EnrichedPeer struct {
	NodeID string `json:"node_id"`
}
