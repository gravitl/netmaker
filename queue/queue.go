package queue

import (
	"context"
	"fmt"

	"github.com/enriquebris/goconcurrentqueue"
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
		logger.Log(2, "initialized queue service!")
		for {
			msg, err := EventQueue.DequeueOrWaitForNextElementContext(ctx)
			if err != nil { // handle dequeue error
				logger.Log(0, "error when dequeuing event -", err.Error())
				continue
			} else { // handle event
				event := msg.(models.Event)
				switch event.Topic {
				case "test":
					fmt.Printf("received test topic event %+v \n", event)
				default:
					fmt.Printf("topic unknown\n")
				}
			}
			logger.Log(0, fmt.Sprintf("queue stats: queued elements %d, openCapacity: %d \n", EventQueue.GetLen(), EventQueue.GetCap()))
		}
	}(ctx)
}

// == private ==
func initQueue() {
	size := servercfg.GetQueueSize()
	if size > 0 {
		EventQueue = goconcurrentqueue.NewFixedFIFO(size)
	} else {
		EventQueue = goconcurrentqueue.NewFIFO()
	}
}
