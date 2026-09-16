package main

import (
	"net/http"

	"github.com/sudip-kandel7/eveCollection/views"
)

func (app *Application) routes() http.Handler {
	mux := new(http.ServeMux)

	mux.Handle("GET /static/",http.FileServerFS(views.Files))
	mux.HandleFunc("GET /", app.home)
	return mux
}
