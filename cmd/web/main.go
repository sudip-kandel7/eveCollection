package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
)

type Application struct {
	logger *slog.Logger
}

func main() {
 logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

	app := Application{
		logger: logger,
	}
	srv := &http.Server{
		Handler: app.routes(),
		ErrorLog: slog.NewLogLogger(logger.Handler(),slog.LevelDebug),
		Addr:    ":8080",
	}
	logger.Info("starting server at ",slog.String("port",":8080"))
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
