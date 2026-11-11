package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/go-playground/form/v4"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sudip-kandel7/eveCollection/internal/db"
	"net/http"
	"strings"
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

func (app *Application) commitSession(w http.ResponseWriter, r *http.Request) error {
	token, expiry, err := app.sessionManager.Commit(r.Context())
	if err != nil {
		return err
	}

	app.sessionManager.WriteSessionCookie(r.Context(), w, token, expiry)
	return nil
}

func (app *Application) contextGetUser(r *http.Request) *db.User {
	user, ok := r.Context().Value(userContextKey).(*db.User)

	if !ok {
		return nil
	}

	return user
}

func generateSignature(message, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func verifySignature(message, signature, secretKey string) bool {
	expected := generateSignature(message, secretKey)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func numericToFloat(n pgtype.Numeric) (float64, error) {
	f, err := n.Float64Value()
	if err != nil {
		return 0, err
	}

	if !f.Valid {
		return 0, errors.New("numeric value is NULL")
	}

	return f.Float64, nil
}
