package main

import "net/http"

func (app *Application) home(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte("rolex World"))
}
