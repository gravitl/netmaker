package controller

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"
)

// HandleRESTRequests - handles the rest requests
func HandleRESTRequests(wg *sync.WaitGroup) {
	defer wg.Done()

	r := mux.NewRouter()

	// Currently allowed dev origin is all. Should change in prod
	// should consider analyzing the allowed methods further
	headersOk := handlers.AllowedHeaders([]string{"Access-Control-Allow-Origin", "X-Requested-With", "Content-Type", "authorization"})
	originsOk := handlers.AllowedOrigins([]string{servercfg.GetAllowedOrigin()})
	methodsOk := handlers.AllowedMethods([]string{"GET", "PUT", "POST", "DELETE"})

	nodeHandlers(r)
	userHandlers(r)
	networkHandlers(r)
	dnsHandlers(r)
	fileHandlers(r)
	serverHandlers(r)
	extClientHandlers(r)

	port := servercfg.GetAPIPort()

	srv := &http.Server{Addr: ":" + port, Handler: handlers.CORS(originsOk, headersOk, methodsOk)(r)}
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Println(err)
		}
	}()
	logic.Log("REST Server successfully started on port "+port+" (REST)", 0)
	c := make(chan os.Signal)

	// Relay os.Interrupt to our channel (os.Interrupt = CTRL+C)
	// Ignore other incoming signals
	signal.Notify(c, os.Interrupt)

	// Block main routine until a signal is received
	// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
	<-c

	// After receiving CTRL+C Properly stop the server
	logic.Log("Stopping the REST server...", 0)
	srv.Shutdown(context.TODO())
	logic.Log("REST Server closed.", 0)
}
