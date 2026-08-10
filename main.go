package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"partitioned-api-throttler/internal/server"
	"partitioned-api-throttler/internal/throttler"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	partitions := flag.Int("partitions", 64, "Number of partitions (shards)")
	limit := flag.Int("limit", 10, "Per-IP request limit")
	window := flag.Duration("window", time.Minute, "Sliding window duration")
	cleanup := flag.Duration("cleanup", 30*time.Second, "Stale-IP cleanup interval")

	flag.Parse()

	t := throttler.New(*partitions, *limit, *window)
	t.StartCleanup(*cleanup)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      server.New(t).Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (partitions=%d limit=%d window=%s)", *addr, *partitions, *limit, *window)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	log.Println("Shutting Down")
}
