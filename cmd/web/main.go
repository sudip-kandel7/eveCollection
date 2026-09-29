package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/alexedwards/scs/v2"
	"github.com/go-playground/form/v4"
	// "github.com/jackc/pgx/v5"
	"time"
	"github.com/joho/godotenv"
	"github.com/sudip-kandel7/eveCollection/internal/db"
)

type Application struct {
	logger *slog.Logger
	queries *db.Queries
	formDecoder    *form.Decoder
	sessionManager *scs.SessionManager
}

func main() {
	
	if err := godotenv.Load(); err != nil {
        log.Fatal("Error loading .env file")
    }
    
 logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

 conn, err := pgxpool.New(context.Background(), os.Getenv("GOOSE_DBSTRING"))
	if err != nil {
		logger.Error("Unable to connect to database: ", slog.String("pg", err.Error()))
		os.Exit(1)
	}
	defer conn.Close()

 if err := conn.Ping(context.Background()); err != nil {
        logger.Error("Ping failed: ", slog.String("pg", err.Error()))
        os.Exit(1)
    }

  formDecoder := form.NewDecoder()

  sessionManager := scs.New()
  sessionManager.Store = pgxstore.New(conn)
  sessionManager.Lifetime = 12 * time.Hour
    
	app := Application{
		logger: logger, 
		queries: db.New(conn),
		formDecoder: formDecoder,
		sessionManager: sessionManager,
	}
	
	srv := &http.Server{
		Handler: app.sessionManager.LoadAndSave(app.LoadUser(app.routes())),
		ErrorLog: slog.NewLogLogger(logger.Handler(),slog.LevelDebug),
		Addr:    ":8080",
	}
	
	logger.Info("starting server at ",slog.String("port",":8080"))
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
	
}
