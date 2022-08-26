package controller

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/servercfg"
)

// HttpHandlers - handler functions for REST interactions
var HttpHandlers = []interface{}{
	nodeHandlers,
	userHandlers,
	networkHandlers,
	dnsHandlers,
	fileHandlers,
	serverHandlers,
	extClientHandlers,
	ipHandlers,
}

// HandleRESTRequests - handles the rest requests
func HandleRESTRequests(wg *sync.WaitGroup) {
	defer wg.Done()

	r := mux.NewRouter()

	// Currently allowed dev origin is all. Should change in prod
	// should consider analyzing the allowed methods further
	headersOk := handlers.AllowedHeaders([]string{"Access-Control-Allow-Origin", "X-Requested-With", "Content-Type", "authorization"})
	originsOk := handlers.AllowedOrigins([]string{servercfg.GetAllowedOrigin()})
	methodsOk := handlers.AllowedMethods([]string{"GET", "PUT", "POST", "DELETE"})

	for _, handler := range HttpHandlers {
		handler.(func(*mux.Router))(r)
	}

	port := servercfg.GetAPIPort()

	srv := &http.Server{Addr: ":" + port, Handler: handlers.CORS(originsOk, headersOk, methodsOk)(r)}
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			logger.Log(0, err.Error())
		}
	}()
	logger.Log(0, "REST Server successfully started on port ", port, " (REST)")

	// Relay os.Interrupt to our channel (os.Interrupt = CTRL+C)
	// Ignore other incoming signals
	ctx, stop := signal.NotifyContext(context.TODO(), os.Interrupt)
	defer stop()

	// Block main routine until a signal is received
	// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
	<-ctx.Done()

	// After receiving CTRL+C Properly stop the server
	logger.Log(0, "Stopping the REST server...")
	logger.Log(0, "REST Server closed.")
	logger.DumpFile(fmt.Sprintf("data/netmaker.log.%s", time.Now().Format(logger.TimeFormatDay)))
	srv.Shutdown(context.TODO())
}
