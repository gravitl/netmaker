package models

import (
        "github.com/vishvananda/netlink"
)

type WireGuardLink struct {
	LinkAttrs *netlink.LinkAttrs
}

func (link *WireGuardLink) Type() string {
	return "wireguard"
}

func (link *WireGuardLink) Attrs() *netlink.LinkAttrs {
	return link.LinkAttrs
}
