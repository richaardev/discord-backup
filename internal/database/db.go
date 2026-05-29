package database

import (
	"database/sql"
	"embed"
	"log/slog"

	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func MustOpen(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		panic(err)
	}

	if err := db.Ping(); err != nil {
		slog.Error("failed to ping database", "err", err)
		panic(err)
	}

	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		slog.Error("failed to set goose dialect", "err", err)
		panic(err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.Error("failed to run migrations", "err", err)
		panic(err)
	}

	slog.Info("database initialized successfully")
	return db
}
