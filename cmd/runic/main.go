package main

import (
	"fmt"
	"os"

	"runic/internal/config"
	"runic/internal/db"
	"runic/internal/executor"
	"runic/internal/server"
	"runic/internal/update"
)

var (
	version = "dev"
	builtAt = ""
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("runic %s", version)
			if builtAt != "" {
				fmt.Printf(" (built %s)", builtAt)
			}
			fmt.Println()
			return
		case "update":
			cmdUpdate()
			return
		case "serve":
			cmdServe()
			return
		}
	}

	printUsage()
}

func cmdUpdate() {
	if update.Repo == "" {
		fmt.Fprintln(os.Stderr, "[update] update command requires a release build with REPO ldflag")
		fmt.Fprintln(os.Stderr, "[update] set BUILD_REPO env var when building, e.g.: BUILD_REPO=gera2ld/runic go run build.go")
		os.Exit(1)
	}

	fmt.Print("[update] checking for updates... ")
	latest, _, err := update.CheckLatest()
	if err != nil {
		fmt.Println()
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		os.Exit(1)
	}
	if latest == "" {
		fmt.Println("no release found")
		os.Exit(1)
	}
	fmt.Println("latest: " + latest)

	if latest == version {
		fmt.Println("[update] already up to date")
		return
	}

	fmt.Printf("[update] upgrading from %s to %s...\n", version, latest)
	if err := update.Install(); err != nil {
		fmt.Fprintf(os.Stderr, "[error] %v\n", err)
		os.Exit(1)
	}
}

func cmdServe() {
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

	runner := executor.NewRunner(cfg)
	sched := executor.NewScheduler(runner, database, cfg.ActionDir, cfg.LogDir)
	sched.Start()
	defer sched.Stop()

	server.Serve(cfg, runner, database, sched)
}

func printUsage() {
	fmt.Printf(`runic %s

Usage:
  runic serve     Start the server
  runic update    Check and install latest release
  runic version   Show version information
`, version)
}
