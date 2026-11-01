package main

import (
	"net/http"

	"github.com/sudip-kandel7/eveCollection/views"
)

func (app *Application) routes() http.Handler {
	mux := new(http.ServeMux)

	mux.Handle("GET /static/", noStore(http.FileServerFS(views.Files)))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))))

	mux.HandleFunc("GET /{$}", app.Home)

	mux.HandleFunc("GET /about", app.About)

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
	mux.Handle("GET /cart", app.requireAuth(http.HandlerFunc(app.Cart)))
	mux.Handle("GET /messages", app.requireAuth(http.HandlerFunc(app.Messages)))
	mux.Handle("GET /reviews", app.requireAuth(http.HandlerFunc(app.Reviews)))

	mux.Handle("GET /admin/dashboard", app.requireAdmin(http.HandlerFunc(app.Dashboard)))
	mux.Handle("POST /admin/categories", app.requireAdmin(http.HandlerFunc(app.Categories)))
	mux.Handle("POST /admin/categories/{id}", app.requireAdmin(http.HandlerFunc(app.UpdateCategory)))
	mux.Handle("POST /admin/categories/{id}/delete", app.requireAdmin(http.HandlerFunc(app.DeleteCategory)))

	mux.Handle("POST /admin/products", app.requireAdmin(http.HandlerFunc(app.Products)))
	mux.Handle("POST /admin/products/{id}", app.requireAdmin(http.HandlerFunc(app.UpdateProduct)))
	mux.Handle("GET /admin/products/search", app.requireAdmin(http.HandlerFunc(app.SearchProducts)))
	mux.Handle("GET /admin/products/{id}/edit", app.requireAdmin(http.HandlerFunc(app.EditProductForm)))

	mux.Handle("GET /products/{id}", http.HandlerFunc(app.ProductDetails))

	mux.Handle("POST /addReview", app.requireAuth(http.HandlerFunc(app.AddReview)))
	mux.Handle("POST /add/cart", app.requireAuth(http.HandlerFunc(app.AddToCart)))

	mux.Handle("GET /products/filter", http.HandlerFunc(app.ProductsFilter))

	return mux

}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
