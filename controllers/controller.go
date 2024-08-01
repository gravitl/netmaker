package controller

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	m "github.com/gravitl/netmaker/migrate"
	"github.com/gravitl/netmaker/servercfg"
)

// HttpMiddlewares - middleware functions for REST interactions
var HttpMiddlewares []mux.MiddlewareFunc

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
	loggerHandlers,
	hostHandlers,
	enrollmentKeyHandlers,
	legacyHandlers,
}

func HandleRESTRequests(wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()

	r := mux.NewRouter()

	// Currently allowed dev origin is all. Should change in prod
	// should consider analyzing the allowed methods further
	headersOk := handlers.AllowedHeaders(
		[]string{
			"Access-Control-Allow-Origin",
			"X-Requested-With",
			"Content-Type",
			"authorization",
			"From-Ui",
		},
	)
	originsOk := handlers.AllowedOrigins(strings.Split(servercfg.GetAllowedOrigin(), ","))
	methodsOk := handlers.AllowedMethods(
		[]string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	)

	for _, middleware := range HttpMiddlewares {
		r.Use(middleware)
	}

	for _, handler := range HttpHandlers {
		handler.(func(*mux.Router))(r)
	}

	port := servercfg.GetAPIPort()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handlers.CORS(originsOk, headersOk, methodsOk)(r),
	}
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			logger.Log(0, err.Error())
		}
	}()
	if os.Getenv("MIGRATE_EMQX") == "true" {
		logger.Log(0, "migrating emqx...")
		time.Sleep(time.Second * 2)
		m.MigrateEmqx()
	}
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
