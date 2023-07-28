package controller

import (
	"context"
	"fmt"
	"github.com/gorilla/handlers"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/servercfg"
)

// FullHttpHandlers - handler functions for REST interactions
var FullHttpHandlers = []func(r *mux.Router){
	nodeHandlers,
	userHandlers,
	networkHandlers,
	dnsHandlers,
	fileHandlers,
	serverHandlers,
	extClientHandlers,
	ipHandlers,
	loggerHandlers,
	hostHandlers,
	enrollmentKeyHandlers,
	legacyHandlers,
}

// LimitedHttpHandlers - limited handler functions for REST interactions
var LimitedHttpHandlers = []func(r *mux.Router){
	serverHandlers,
}

// HandleRESTRequests - handles the rest requests
func HandleRESTRequests(wg *sync.WaitGroup, ctx context.Context, httpHandlers []func(r *mux.Router)) {
	defer wg.Done()

	r := mux.NewRouter()

	// Currently allowed dev origin is all. Should change in prod
	// should consider analyzing the allowed methods further
	headersOk := handlers.AllowedHeaders([]string{"Access-Control-Allow-Origin", "X-Requested-With", "Content-Type", "authorization"})
	originsOk := handlers.AllowedOrigins(strings.Split(servercfg.GetAllowedOrigin(), ","))
	methodsOk := handlers.AllowedMethods([]string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete})

	for _, handler := range httpHandlers {
		handler(r)
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

	// Block main routine until a signal is received
	// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
	<-ctx.Done()
	// After receiving CTRL+C Properly stop the server
	logger.Log(0, "Stopping the REST server...")
	if err := srv.Shutdown(context.TODO()); err != nil {
		logger.Log(0, "REST shutdown error occurred -", err.Error())
	}
	logger.Log(0, "REST Server closed.")
	logger.DumpFile(fmt.Sprintf("data/netmaker.log.%s", time.Now().Format(logger.TimeFormatDay)))
}
