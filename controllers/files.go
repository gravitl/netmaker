package controller

import (
	"net/http"

	"github.com/gorilla/mux"
)

func fileHandlers(r *mux.Router) {
	// swagger:route GET /meshclient/files/{filename} meshclient fileServer
	//
	// Retrieve a file from the file server
	//
	//		Schemes: https
	//
	// 		Security:
	//   		oauth
	r.PathPrefix("/meshclient/files").Handler(http.StripPrefix("/meshclient/files", http.FileServer(http.Dir("./meshclient/files"))))
}
