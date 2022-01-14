package models

type PeerUpdate struct {
	Network  string
	Nodes    []Node
	ExtPeers []ExtPeersResponse
}

type KeyUpdate struct {
	Network   string
	Interface string
}
