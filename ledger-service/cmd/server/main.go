package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ledger-service/internal/domain"
	apphttp "ledger-service/internal/http"
	"ledger-service/internal/store"
)

func getDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	// Fall back to reading .env file if available
	data, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DATABASE_URL=") {
			val := strings.TrimPrefix(line, "DATABASE_URL=")
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}

func main() {
	mux := http.NewServeMux()

	databaseURL := getDatabaseURL()
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	dbCtx, dbCancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer dbCancel()

	pgStore, err := store.NewPostgresStore(
		dbCtx,
		databaseURL,
	)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	defer pgStore.Close()

	transferRepo := store.NewPostgresTransferRepository(pgStore)
	idempotencyRepo := store.NewPostgresIdempotencyRepository(pgStore)

	notifier := domain.NoopNotifier{}

	transferService := domain.NewTransferService(
		transferRepo,
	).
		WithIdempotency(idempotencyRepo).
		WithNotifier(notifier)

	handler := apphttp.NewHandler(transferService)

	mux.HandleFunc("/health", handler.Health)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Root context for the application.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		log.Println("ledger-service listening on :8080")

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Wait until SIGINT / SIGTERM.
	<-ctx.Done()

	log.Println("shutdown signal received")

	// Give active requests time to finish.
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	log.Println("server stopped")
}
