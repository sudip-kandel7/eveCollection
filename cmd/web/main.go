package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
)

type Application struct {
	logger *log.Logger
}

func main() {

	app := Application{
		logger: slog.NewLogLogger(slog.NewJSONHandler(os.Stdout, nil), slog.LevelDebug),
	}
	srv := &http.Server{
		Handler: app.Routes(),
		Addr:    ":8080",
	}
	fmt.Println("Starting server on port 8080")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
