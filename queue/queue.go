package queue

import (
	"context"
	"errors"
	"fmt"

	"github.com/enriquebris/goconcurrentqueue"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// EventQueue - responsible for queueing and handling events sent to the server
var EventQueue goconcurrentqueue.Queue

// StartQueue - starts the queue and listens for messages
func StartQueue(ctx context.Context) {
	initQueue()

	go func(ctx context.Context) {
		for {
			msg, err := EventQueue.DequeueOrWaitForNextElementContext(ctx)
			if err != nil { // handle dequeue error
				if errors.Is(err, context.Canceled) {
					logger.Log(0, "queue shutdown successfully")
					break
				}
				logger.Log(0, "error dequeuing event -", err.Error())
				continue
			}
			event := msg.(models.Event)
			switch event.Topic {
			case models.EventTopics.Test:
				conn, ok := ConnMap.Load(event.ID)
				if ok {
					conn.(*websocket.Conn).WriteMessage(websocket.TextMessage, []byte("success"))
				}
			default:
				logger.Log(0, fmt.Sprintf("received an unknown topic %d \n", event.Topic))
			}
			logger.Log(3, fmt.Sprintf("queue stats: queued elements %d, openCapacity: %d \n", EventQueue.GetLen(), EventQueue.GetCap()))
		}
	}(ctx)
}

// == private ==
func initQueue() {
	size := servercfg.GetQueueSize()
	if size > 0 {
		logger.Log(0, "started queue with fixed allocation -", fmt.Sprintf("%d", size))
		EventQueue = goconcurrentqueue.NewFixedFIFO(size)
	} else {
		logger.Log(0, "started queue with dynamic allocation")
		EventQueue = goconcurrentqueue.NewFIFO()
	}
}
