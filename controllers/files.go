package controller

import (
	"github.com/gorilla/mux"
	"net/http"
)

func fileHandlers(r *mux.Router) {
	r.PathPrefix("/meshclient/files").Handler(http.StripPrefix("/meshclient/files", http.FileServer(http.Dir("./meshclient/files"))))
}
