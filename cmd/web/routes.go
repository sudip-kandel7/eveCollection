package main

import "net/http"

func (app *Application) Routes() http.Handler {
	mux := new(http.ServeMux)
	mux.HandleFunc("GET /", app.home)
	return mux
}
