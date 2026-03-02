package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"agent-relay/internal/cli"
	"agent-relay/internal/db"
	"agent-relay/internal/relay"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Printf("agent-relay %s\n", Version)
			return
		case "--help", "-h":
			cli.Run([]string{"help"})
			return
		case "serve":
			startServer()
			return
		case "status", "agents", "inbox", "send", "thread", "stats", "conversations":
			cli.Run(os.Args[1:])
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
			cli.Run([]string{"help"})
			os.Exit(1)
		}
	}

	// No args → start server (backward compat).
	startServer()
}

func startServer() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("agent-relay starting...")

	database, err := db.New()
	if err != nil {
		log.Fatalf("failed to init database: %v", err)
	}
	defer database.Close()

	r := relay.New(database)

	addr := ":8090"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start stale agent cleanup goroutine.
	cleanupDone := make(chan struct{})
	relay.StartCleanup(database, cleanupDone)

	go func() {
		log.Printf("listening on %s", addr)
		if err := r.HTTP.Start(addr); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	<-ctx.Done()
	close(cleanupDone)
	log.Println("shutting down...")
	if err := r.HTTP.Shutdown(context.Background()); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
