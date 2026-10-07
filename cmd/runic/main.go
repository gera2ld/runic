package main

import (
	"fmt"
	"os"

	"runic/internal/config"
	"runic/internal/db"
	"runic/internal/executor"
	"runic/internal/server"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("runic %s\n", version)
			return
		case "serve":
			cmdServe(os.Args[2:])
			return
		}
	}

	printUsage()
}

func resolveConfigPath(args []string) (string, bool, error) {
	path := os.Getenv("RUNIC_CONFIG")
	explicit := path != ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config", "-c":
			if i+1 >= len(args) {
				return "", false, fmt.Errorf("flag %s requires a value", args[i])
			}
			path = args[i+1]
			explicit = true
			i++
		default:
			return "", false, fmt.Errorf("unknown argument: %s", args[i])
		}
	}
	if path == "" {
		path = "config.yml"
	}
	return path, explicit, nil
}

func cmdServe(args []string) {
	configPath, explicit, err := resolveConfigPath(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		printUsage()
		os.Exit(1)
	}
	if explicit {
		if _, err := os.Stat(configPath); err != nil {
			fmt.Fprintf(os.Stderr, "[error] config file not found: %s\n", configPath)
			os.Exit(1)
		}
	}

	cfg, err := config.Load(configPath)
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

	runner := executor.NewRunner(cfg, database)
	sched := executor.NewScheduler(runner, database, cfg.LogDir)
	sched.Start()
	defer sched.Stop()

	server.Serve(cfg, runner, database, sched)
}

func printUsage() {
	fmt.Printf(`runic %s

Usage:
  runic serve [--config <path>]   Start the server (default config: config.yml)
  runic version                   Show version information
`, version)
}
