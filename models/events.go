package models

// Event - holds info about messages to be used by different handlers
type Event struct {
	ID      string `json:"id"`
	Topic   int    `json:"topic"`
	Payload struct {
		*Host `json:"host,omitempty"`
		*Node `json:"odd,omitempty"`
		*Test `json:"test,omitempty"`
	} `json:"payload"`
}

// Test - used for testing the handlers
type Test struct {
	Data string `json:"data"`
}

// == TOPICS ==

// EventTopics - hold topic IDs for each type of possible event
var EventTopics = struct {
	Test       int
	NodeUpdate int
	HostUpdate int
	PeerUpdate int
}{
	Test:       0,
	NodeUpdate: 1,
	HostUpdate: 2,
	PeerUpdate: 3,
}
