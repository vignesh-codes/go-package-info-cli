// Package main provides a command-line tool for analyzing Debian package statistics.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	app "github.com/canonical-dev/package_statistics/internal/app"
)

// main is the entry point for the package_statistics command-line tool.
func main() {
	cfg, err := app.ParseFlags()
	if err != nil {
		log.Fatalf("invalid args: %v", err)
	}

	if err := os.MkdirAll(cfg.CacheDir, 0o755); err != nil {
		log.Fatalf("failed to create cache dir: %v", err)
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()
	}()

	a := app.NewApp(cfg, nil)
	stats, err := a.AnalyzeWithCache(ctx)
	if err != nil {
		if ctx.Err() == context.Canceled {
			log.Println("Operation cancelled")
			os.Exit(130) // Standard exit code for Ctrl+C
		}
		log.Fatalf("analysis failed: %v", err)
	}

	app.PrintTop(stats, cfg.TopCount)
}
