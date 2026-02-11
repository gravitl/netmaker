package controller

import (
	"net/http"

	"github.com/gorilla/mux"
)

func fileHandlers(r *mux.Router) {
	r.PathPrefix("/meshclient/files").
		Handler(http.StripPrefix("/meshclient/files", http.FileServer(http.Dir("./meshclient/files"))))
}
