package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/nhdewitt/spectra/internal/database"
	"github.com/nhdewitt/spectra/internal/server"
)

func main() {
	dbURL := flag.String("db", "", "PostgreSQL connection string")
	port := flag.Int("port", 8080, "Server port")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = os.Getenv("DATABASE_URL")
	}
	if *dbURL == "" {
		log.Fatal("Database URL required: use -db flag or DATABASE_URL env var")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, *dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	queries := database.New(pool)

	cfg := server.Config{
		Port: *port,
	}

	srv := server.New(cfg, queries)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
