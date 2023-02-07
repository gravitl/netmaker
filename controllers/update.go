package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/queue"
)

func updateHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/update", http.HandlerFunc(handleUpdate)).Methods(http.MethodGet)
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log(0,
			fmt.Sprintf("error occurred starting update ws for a client [%v]", err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if len(r.Header.Get(hostID)) > 0 {
		queue.ConnMap.Store(r.Header.Get(hostID), c)
	} else {
		queue.ConnMap.Store("test", c)
	}
	// load the connection address for reference later
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
		fmt.Printf("got event: %+v \n", event)
		queue.EventQueue.Enqueue(event)
	}
	if len(r.Header.Get(hostID)) > 0 {
		queue.ConnMap.Delete(r.Header.Get(hostID))
	} else {
		queue.ConnMap.Delete("test")
	}
}
