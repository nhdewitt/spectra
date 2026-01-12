package main

import (
	"log"

	"github.com/nhdewitt/spectra/internal/server"
)

func main() {
	cfg := server.Config{
		Port: 8080,
	}

	srv := server.New(cfg)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
