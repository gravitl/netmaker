package queue

import (
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/models"
)

// holds a map of funcs
// based on topic to handle an event
var handlerFuncs map[int]func(*models.Event)

// initializes the map of functions
func initializeHandlers() {
	handlerFuncs = make(map[int]func(*models.Event))
	handlerFuncs[models.EventTopics.NodeUpdate] = nodeUpdate
	handlerFuncs[models.EventTopics.Test] = test
}

func test(e *models.Event) {
	val, ok := ConnMap.Load(e.ID)
	if ok {
		conn := val.(*websocket.Conn)
		if conn != nil {
			conn.WriteMessage(websocket.TextMessage, []byte("success"))
		}
	}
}

func nodeUpdate(e *models.Event) {

}
