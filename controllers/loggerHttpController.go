package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
)

func loggerHandlers(r *mux.Router) {
	r.HandleFunc("/api/logs", securityCheckDNS(true, true, http.HandlerFunc(getLogs))).Methods("GET")
}

func getLogs(w http.ResponseWriter, r *http.Request) {
	var currentTime = time.Now().Format(logger.TimeFormatDay)
	var currentFilePath = fmt.Sprintf("data/netmaker.log.%s", currentTime)
	logger.DumpFile(currentFilePath)
	logger.ResetLogs()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(logger.Retrieve(currentFilePath)))
}
