//go:build debug

package controller

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"

	"github.com/gravitl/netmaker/logger"
)

func init() {
	srv := &http.Server{Addr: "0.0.0.0:6060", Handler: nil}
	go func() {
		logger.Log(0, "Debug mode active")
		err := srv.ListenAndServe()
		if err != nil {
			logger.Log(0, err.Error())
		}
		c := make(chan os.Signal)

		// Relay os.Interrupt to our channel (os.Interrupt = CTRL+C)
		// Ignore other incoming signals
		signal.Notify(c, os.Interrupt)
		// Block main routine until a signal is received
		// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
		<-c
		srv.Shutdown(context.TODO())
	}()
}
