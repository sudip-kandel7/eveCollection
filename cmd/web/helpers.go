package main

import (
	"errors"
	"net/http"

	"github.com/go-playground/form/v4"
	"github.com/sudip-kandel7/eveCollection/internal/db"
)

func (app *Application) decodePostForm(r *http.Request, dst any) error {
	err := r.ParseForm()
	if err != nil {
		return err
	}

	err = app.formDecoder.Decode(dst, r.PostForm)
	if err != nil {
		var invalidDecoderError *form.InvalidDecoderError
		if errors.As(err, &invalidDecoderError) {
			panic(err)
		}
		return err
	}

	return nil
}

func (app *Application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error(err.Error(),
		"method", r.Method,
		"uri", r.URL.RequestURI(),
	)
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func (app *Application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}


func (app *Application) contextGetUser(r *http.Request) *db.User {
    user, ok := r.Context().Value(userContextKey).(*db.User)

    if !ok {
        return nil
    }

    return user
}