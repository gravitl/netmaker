package models

import "github.com/gorilla/websocket"

// Event - holds info about messages to be used by different handlers
type Event struct {
	ID      string `json:"id"`
	Topic   string `json:"topic"`
	Payload struct {
		*Host `json:"host,omitempty"`
		*Node `json:"odd,omitempty"`
		*Test `json:"test,omitempty"`
	} `json:"payload"`
	Conn *websocket.Conn `json:"conn"`
}

// Test - used for testing the handlers
type Test struct {
	Data string `json:"data"`
}

// == TOPICS ==
const (
	// Event_TestTopic - the topic for a test event
	Event_TestTopic = "test"
)
