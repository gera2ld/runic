package main

import (
	"fmt"
	"os"

	"github.com/gera2ld/runic/internal/config"
	"github.com/gera2ld/runic/internal/db"
	"github.com/gera2ld/runic/internal/executor"
	"github.com/gera2ld/runic/internal/server"
)

func main() {
	cfg, err := config.Load("config.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] invalid config: %v\n", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	server.Serve(cfg, executor.NewRunner(cfg), database)
}
