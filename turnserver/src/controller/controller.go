package controller

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/handlers"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/turnserver/config"
	"github.com/gravitl/netmaker/turnserver/src/middleware"
	"github.com/gravitl/netmaker/turnserver/src/routes"
)

// HandleRESTRequests - handles the rest requests
func HandleRESTRequests(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	// Set GIN MODE (debug or release)
	gin.SetMode(gin.ReleaseMode)
	if config.GetVersion() == "dev" || config.IsDebugMode() {
		gin.SetMode(gin.DebugMode)
	}
	// Register gin router with default configuration
	// comes with default middleware and recovery handlers.
	router := gin.Default()
	// Intialize all routes
	routes.Init(router)
	// Attach custom logger to gin to print incoming requests to stdout.
	router.Use(ginLogger())
	// Attach rate limiter to middleware
	router.Use(middleware.RateLimiter())
	// Currently allowed dev origin is all. Should change in prod
	// should consider analyzing the allowed methods further
	headersOk := handlers.AllowedHeaders([]string{"Access-Control-Allow-Origin", "X-Requested-With", "Content-Type", "authorization"})
	originsOk := handlers.AllowedOrigins([]string{config.GetAllowedOrigin()})
	methodsOk := handlers.AllowedMethods([]string{"GET", "PUT", "POST", "DELETE"})

	// get server port from config
	port := config.GetAPIPort()
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handlers.CORS(originsOk, headersOk, methodsOk)(router),
	}
	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	logger.Log(0, fmt.Sprintf("REST Server (Version: %s) successfully started on port (%d) ", config.GetVersion(), port))
	<-ctx.Done()
	log.Println("Shutdown Server ...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	logger.Log(0, "Server exiting...")
	os.Exit(0)
}

func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {

		// Get the client IP address
		clientIP := c.ClientIP()

		// Get the current time
		now := time.Now()
		// Log the request
		log.Printf("[%s] %s %s %s", now.Format(time.RFC3339), c.Request.Method, c.Request.URL.Path, clientIP)

		// Proceed to the next handler
		c.Next()
	}
}
