package main

import (
	"net/http"

	"github.com/sudip-kandel7/eveCollection/views"
)

func (app *Application) home(w http.ResponseWriter, r *http.Request) {
	views.IndexRenderer().Render(r.Context(),w)
}
