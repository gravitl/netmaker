package models

// Event - holds info about messages to be used by different handlers
type Event struct {
	ID      string `json:"id"`
	Topic   int    `json:"topic"`
	Payload struct {
		*HostUpdate     `json:"host,omitempty"`
		*Node           `json:"node,omitempty"`
		*Test           `json:"test,omitempty"`
		*NodeCheckin    `json:"node_checkin,omitempty"`
		*Metrics        `json:"metrics,omitempty"`
		*HostPeerUpdate `json:"host_peer_update,omitempty"`
		Action          byte `json:"action"`
	} `json:"payload"`
}

// Test - just used for testing the handlers
type Test struct {
	Data string `json:"data"`
}

// == TOPICS ==

// EventTopics - hold topic IDs for each type of possible event
var EventTopics = struct {
	Test                  int
	NodeUpdate            int
	HostUpdate            int
	Ping                  int
	Metrics               int
	ClientUpdate          int
	SendAllHostPeerUpdate int
	SendHostPeerUpdate    int
}{
	Test:                  0,
	NodeUpdate:            1,
	HostUpdate:            2,
	Ping:                  3,
	Metrics:               4,
	ClientUpdate:          5,
	SendAllHostPeerUpdate: 6,
	SendHostPeerUpdate:    7,
}
