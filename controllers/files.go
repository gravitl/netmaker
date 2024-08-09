package controller

import (
	"net/http"

	"github.com/gorilla/mux"
)

// @Summary     Retrieve a file from the file server
// @Router      /meshclient/files/{filename}  [get]
// @Tags        Meshclient
// @Success     200 {body} file "file"
// @Failure     404 {string} string "404 not found"
func fileHandlers(r *mux.Router) {
	r.PathPrefix("/meshclient/files").
		Handler(http.StripPrefix("/meshclient/files", http.FileServer(http.Dir("./meshclient/files"))))
}
