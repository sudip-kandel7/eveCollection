package main

import (
	"context"
	"net/http"

	"github.com/sudip-kandel7/eveCollection/internal/db"
)

type contextKey string
const userContextKey contextKey = "user"


func (app *Application) LoadUser(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

        userID := app.sessionManager.GetInt32(r.Context(), "authenticatedUserID")

        if userID == 0 {
            next.ServeHTTP(w, r)
            return
        }

        row, err := app.queries.GetUserById(r.Context(), userID)
        if err != nil {
            app.logger.Error("failed to get user", "error", err)
            app.serverError(w, r, err)
            return
        }

        user := &db.User{
            ID:        row.ID,
            Name:      row.Name,
            Email:     row.Email,
            Phone:     row.Phone,
            Role:      row.Role,
            Address:   row.Address,
            IsActive:  row.IsActive,
            CreatedAt: row.CreatedAt,
            UpdatedAt: row.UpdatedAt,
        }

        ctx := context.WithValue(
            r.Context(),
            userContextKey,
            user,
        )

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func (app *Application) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := app.sessionManager.GetInt32(r.Context(), "authenticatedUserID")
		
		if userID == 0 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *Application) requireAdmin(next http.Handler) http.Handler {
	return app.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := app.sessionManager.GetInt32(r.Context(), "authenticatedUserID")
		role, err := app.queries.GetUserRole(r.Context(), userID)
		if err != nil || role != "admin" {
			app.clientError(w, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	}))
}