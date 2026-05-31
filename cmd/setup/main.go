package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nhdewitt/spectra/internal/setup"
)

func main() {
	configPath := flag.String("config", setup.DefaultConfigPath, "Path to save server config")
	fromFile := flag.String("from", "", "Path to YAML setup file for non-interactive setup")
	flag.Parse()

	ctx := context.Background()

	if *fromFile != "" {
		sf, err := setup.LoadSetupFile(*fromFile)
		if err != nil {
			log.Fatalf("Invalid setup file: %v", err)
		}
		if err := setup.RunNonInteractive(ctx, sf, *configPath); err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		return
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("Spectra Server Setup")
	fmt.Println("====================")
	fmt.Println()

	local := setup.PromptYesNo(reader, "Use local database", true)
	dbCfg := setup.PromptDBConfig(reader, local)

	migrationsDir := setup.DefaultMigrationsPath
	if _, err := os.Stat(migrationsDir); err != nil {
		fmt.Printf("  [!] Default migrations path not found: %s\n", migrationsDir)
		migrationsDir = setup.PromptMigrationsDir(reader)
	}

	admin := setup.PromptAdmin(reader)
	port := setup.PromptPort(reader)
	tlsCfg := setup.PromptTLS(reader)
	externalURL := setup.PromptExternalURL(reader, port, tlsCfg != nil)

	sc := &setup.SetupConfig{
		DBConfig:      dbCfg,
		CreateDB:      local,
		MigrationsDir: migrationsDir,
		Admin:         admin,
		Port:          port,
		TLS:           tlsCfg,
		ExternalURL:   externalURL,
	}

	if err := setup.RunSetup(ctx, sc, *configPath); err != nil {
		log.Fatalf("Setup failed: %v", err)
	}
}
