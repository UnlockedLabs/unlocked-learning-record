// Command app is Learning Record's one binary: "app serve" runs the API and
// "app migrate" migrates the database. For a local database, "app
// migrate-reset" rolls back the migrations. serve stops cleanly on SIGINT or
// SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/UnlockedLabs/unlocked-contracts/identity"
	"github.com/UnlockedLabs/unlocked-learning-record/internal/database"
	"github.com/UnlockedLabs/unlocked-learning-record/internal/handlers"
	"github.com/UnlockedLabs/unlocked-learning-record/internal/server"
	log "github.com/sirupsen/logrus"
)

const usage = "usage: app serve | app migrate | app migrate-reset"

func main() {
	initLogging()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	err := run(ctx, os.Args[1:])
	// A stop while it waits for the migrations is a clean stop.
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		log.Info("Stopped before it started")
		return
	}
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "serve":
		return serve(ctx)
	case "migrate":
		return migrate(ctx)
	case "migrate-reset":
		return migrateReset(ctx)
	default:
		return fmt.Errorf("unknown command %q: %s", args[0], usage)
	}
}

// serve runs the API once the database is migrated. KRATOS_PUBLIC_URL says
// whether a login session is active, and core, at APP_URL, who its user is,
// including which features are on at their facility.
func serve(ctx context.Context) error {
	db, err := connectDB(ctx)
	if err != nil {
		return err
	}
	defer closeDB(db)
	sessions := identity.NewSessionChecker(os.Getenv("KRATOS_PUBLIC_URL"), os.Getenv("APP_URL"), nil)
	app := handlers.NewHandler(db, sessions.Require)
	return server.Serve(ctx, envOr("ADDR", ":8080"), os.Getenv("PATH_PREFIX"), server.VersionFromEnv(), app)
}

// migrate runs Learning Record's migrations as the database's own user.
func migrate(ctx context.Context) error {
	owner, err := database.Open(ctx, database.DSN("prefer"))
	if err != nil {
		return err
	}
	defer func() { _ = owner.Close() }()
	if err := database.Migrate(ctx, owner); err != nil {
		return err
	}
	log.Info("Migrations completed successfully")
	return nil
}

// migrateReset rolls back Learning Record's migrations, for core's
// `make migrate-fresh`. It runs only with APP_ENV=dev, so it can never touch
// a real site.
func migrateReset(ctx context.Context) error {
	if os.Getenv("APP_ENV") != "dev" {
		return errors.New("migrate-reset rolls back the migrations, so it runs only with APP_ENV=dev")
	}
	owner, err := database.Open(ctx, database.DSN("prefer"))
	if err != nil {
		return err
	}
	defer func() { _ = owner.Close() }()
	return database.Reset(ctx, owner)
}

// connectDB waits until the database is migrated, then opens Learning
// Record's connection to it, as the learning_record role.
func connectDB(ctx context.Context) (*database.DB, error) {
	owner, err := database.Open(ctx, database.DSN("allow"))
	if err != nil {
		return nil, err
	}
	err = database.WaitForMigrations(ctx, owner)
	_ = owner.Close()
	if err != nil {
		return nil, fmt.Errorf("wait for migrations: %w", err)
	}
	db, err := database.OpenAs(ctx, database.DSN("allow"), database.Schema)
	if err != nil {
		return nil, err
	}
	return database.NewDB(db), nil
}

func closeDB(db *database.DB) {
	if sqlDB, err := db.DB.DB(); err == nil {
		if err := sqlDB.Close(); err != nil {
			log.Errorf("close database: %v", err)
		}
	}
}

// initLogging logs JSON at LOG_LEVEL (info by default), as core does.
func initLogging() {
	log.SetFormatter(&log.JSONFormatter{})
	level, err := log.ParseLevel(envOr("LOG_LEVEL", "info"))
	if err != nil {
		log.Errorf("LOG_LEVEL: %v; logging at info", err)
		level = log.InfoLevel
	}
	log.SetLevel(level)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
