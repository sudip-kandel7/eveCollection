package main

import (
	"net/http"

	"github.com/sudip-kandel7/eveCollection/views"
)

func (app *Application) routes() http.Handler {
	mux := new(http.ServeMux)

	mux.Handle("GET /static/",http.FileServerFS(views.Files))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/",http.FileServer(http.Dir("./uploads"))))

	
	mux.HandleFunc("GET /", app.Home)
	
	mux.HandleFunc("GET /login", app.Login)
	mux.HandleFunc("POST /login", app.Signin)
	
	mux.HandleFunc("GET /register", app.Register)
	mux.HandleFunc("POST /register", app.Signup)
	
	mux.HandleFunc("GET /admin/login", app.AdminLogin)
	mux.HandleFunc("POST /admin/login", app.AdminSignin)
	
	mux.Handle("POST /logout", app.requireAuth(http.HandlerFunc(app.Logout)))
	
	mux.Handle("GET /profile", app.requireAuth(http.HandlerFunc(app.Profile)))
	mux.Handle("POST /profile", app.requireAuth(http.HandlerFunc(app.updateProfile)))

	mux.Handle("GET /orders", app.requireAuth(http.HandlerFunc(app.Orders)))
	mux.Handle("GET /messages", app.requireAuth(http.HandlerFunc(app.Messages)))
	mux.Handle("GET /reviews", app.requireAuth(http.HandlerFunc(app.Reviews)))


	mux.Handle("GET /admin/dashboard", app.requireAdmin(http.HandlerFunc(app.Dashboard)))
	mux.Handle("POST /admin/categories", app.requireAdmin(http.HandlerFunc(app.Categories)))
	mux.Handle("POST /admin/categories/{id}", app.requireAdmin(http.HandlerFunc(app.UpdateCategory)))
	mux.Handle("POST /admin/categories/{id}/delete", app.requireAdmin(http.HandlerFunc(app.DeleteCategory)))

	
	return mux
	
}
