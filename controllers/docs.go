package controller

import (
	"net/http"

	"github.com/gorilla/mux"
)

func docsHandler(r *mux.Router) {
	r.PathPrefix("/docs").Handler(http.StripPrefix("/docs", http.FileServer(http.Dir("./docs"))))
}
