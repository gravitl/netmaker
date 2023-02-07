package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/queue"
)

var updateUpgrader = websocket.Upgrader{} // use default options

func updateHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/update", http.HandlerFunc(handleUpdate)).Methods(http.MethodGet)
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	c, err := updateUpgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log(0,
			fmt.Sprintf("error occurred starting update ws for a client [%v]", err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	defer c.Close()
	for {
		_, msg, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		var event models.Event
		err = json.Unmarshal(msg, &event)
		if err != nil {
			log.Printf("error unmarshalling json! %v\n", err)
			continue
		}
		event.Conn = c
		fmt.Printf("got event: %+v \n", event)
		queue.EventQueue.Enqueue(event)
	}
}
