package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/turnserver/src/controller"
	"github.com/gravitl/netmaker/turnserver/src/turn"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 2)
	// kill (no param) default send syscanll.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but cant be caught, so don't need add it
	wg.Add(1)
	go controller.HandleRESTRequests(ctx, wg)
	wg.Add(1)
	go turn.Start(ctx, wg)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log(0, "Recieved Shutdown Signal...")
	cancel()
	wg.Wait()
	logger.Log(0, "Stopping Turn Server...")
}
