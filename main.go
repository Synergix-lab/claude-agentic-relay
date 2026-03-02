package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"agent-relay/internal/db"
	"agent-relay/internal/relay"
)

func main() {
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

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on %s", addr)
		if err := r.HTTP.Start(addr); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	if err := r.HTTP.Shutdown(context.Background()); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
