package controller

import (
    "github.com/gravitl/netmaker/mongoconn"
    "os/signal"
    "os"
    "fmt"
    "context"
    "net/http"
    "github.com/gorilla/mux"
    "github.com/gorilla/handlers"
    "sync"
    "github.com/gravitl/netmaker/config"
)


func HandleRESTRequests(wg *sync.WaitGroup) {
    defer wg.Done()

    r := mux.NewRouter()

    // Currently allowed dev origin is all. Should change in prod
    // should consider analyzing the allowed methods further
    headersOk := handlers.AllowedHeaders([]string{"Access-Control-Allow-Origin", "X-Requested-With", "Content-Type", "authorization"})
    originsOk := handlers.AllowedOrigins([]string{config.Config.Server.AllowedOrigin})
    methodsOk := handlers.AllowedMethods([]string{"GET", "PUT", "POST", "DELETE"})

    nodeHandlers(r)
    userHandlers(r)
    groupHandlers(r)
    fileHandlers(r)

		port := config.Config.Server.ApiPort
	        if os.Getenv("API_PORT") != "" {
			port = os.Getenv("API_PORT")
		}

		srv := &http.Server{Addr: ":" + port, Handler: handlers.CORS(originsOk, headersOk, methodsOk)(r)}
		go func(){
		err := srv.ListenAndServe()
		//err := http.ListenAndServe(":" + port,
		//handlers.CORS(originsOk, headersOk, methodsOk)(r))
		if err != nil {
			fmt.Println(err)
		}
		}()
		fmt.Println("REST Server succesfully started on port " + port + " (REST)")
		c := make(chan os.Signal)

		// Relay os.Interrupt to our channel (os.Interrupt = CTRL+C)
		// Ignore other incoming signals
		signal.Notify(c, os.Interrupt)

		// Block main routine until a signal is received
		// As long as user doesn't press CTRL+C a message is not passed and our main routine keeps running
		<-c

		// After receiving CTRL+C Properly stop the server
		fmt.Println("Stopping the REST server...")
		srv.Shutdown(context.TODO())
                fmt.Println("REST Server closed.")
		mongoconn.Client.Disconnect(context.TODO())
}
