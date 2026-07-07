// Command migrate applies (or rolls back) the SQL files in /migrations
// against DATABASE_DSN. A thin wrapper around golang-migrate so scripts/dev.sh
// doesn't require the separate `migrate` CLI to be installed — the Go
// toolchain the project already needs is enough.
//
// Usage: go run ./cmd/migrate up|down|version
package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: go run ./cmd/migrate up|down|version")
	}
	cmd := os.Args[1]

	if err := godotenv.Load(); err != nil {
		log.Println(".env not found, relying on process environment")
	}
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		log.Fatal("DATABASE_DSN is not set")
	}

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		log.Fatalf("migrate init: %v", err)
	}
	defer m.Close()

	switch cmd {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "version":
		version, dirty, verr := m.Version()
		if verr != nil {
			log.Fatalf("version: %v", verr)
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)
		return
	default:
		log.Fatalf("unknown command: %s (expected up, down or version)", cmd)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("migrate %s: %v", cmd, err)
	}
	log.Printf("migrate %s: done", cmd)
}
