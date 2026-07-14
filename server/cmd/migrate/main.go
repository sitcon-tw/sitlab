package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"example.com/project-template/internal/controller/config"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	command := flags.String("command", "status", "migration command: status, up, or down")
	directory := flags.String("dir", "db/migrations", "migration directory")
	databaseOverride := flags.String("database-url", "", "override SITCON_BOARD_DATABASE_URL")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	databaseURL := *databaseOverride
	if databaseURL == "" {
		var err error
		databaseURL, err = config.LoadDatabaseURL()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "load database config: %v\n", err)
			return 2
		}
	}
	if err := goose.SetDialect("postgres"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "set migration dialect: %v\n", err)
		return 1
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		return 1
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ping database: %v\n", err)
		return 1
	}
	switch *command {
	case "status":
		err = goose.Status(db, *directory)
	case "up":
		err = goose.Up(db, *directory)
	case "down":
		err = goose.Down(db, *directory)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unsupported migration command %q\n", *command)
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
		return 1
	}
	return 0
}
