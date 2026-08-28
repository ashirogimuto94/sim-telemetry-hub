package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"simtelemetry-hub/internal/config"
	"simtelemetry-hub/internal/handler"
	"simtelemetry-hub/internal/repository"
	"simtelemetry-hub/internal/service"
	"simtelemetry-hub/pkg/database"
)

func main() {
	log.Println("Starting SimTelemetry Hub service...")

	// 1. Load Configuration
	cfg := config.Load()

	// 2. Initialize Database Connection Pool
	dbConfig := database.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	}

	db, err := database.NewPostgresDB(dbConfig)
	if err != nil {
		log.Fatalf("Fatal database error: %v", err)
	}
	defer db.Close()
	log.Println("PostgreSQL connection established successfully.")

	// 3. Initialize Repository Layer
	repo := repository.NewPostgresRepository(db)

	// 4. Initialize Worker Pool
	workerPool := service.NewWorkerPool(cfg.WorkerPoolSize, cfg.JobQueueBuffer, repo)
	workerPool.Start()

	// 5. Initialize Service & Handlers
	svc := service.NewTelemetryService(repo, workerPool)
	h := handler.NewTelemetryHandler(svc, repo)

	// 6. Setup Chi Router & Middlewares
	r := chi.NewRouter()
	r.Use(handler.RecoveryMiddleware)
	r.Use(handler.LoggerMiddleware)
	r.Use(handler.JSONMiddleware)

	// Register API v1 Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/telemetry", h.IngestTelemetry)
		r.Get("/leaderboard", h.GetLeaderboard)
		r.Get("/health", h.HealthCheck)
	})

	// 7. Setup HTTP Server
	serverAddr := fmt.Sprintf(":%s", cfg.ServerPort)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 8. Run Server in Goroutine
	go func() {
		log.Printf("HTTP Server listening on port %s...", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failure: %v", err)
		}
	}()

	// 9. Graceful Shutdown Signal Intercept (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Received termination signal. Initiating graceful shutdown...")

	// Create context with 15 second timeout for server & worker drain
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Shut down HTTP server first to stop accepting new requests
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server forced shutdown: %v", err)
	} else {
		log.Println("HTTP server closed cleanly.")
	}

	// Stop worker pool & wait for active tasks to flush
	workerPool.Stop()

	log.Println("SimTelemetry Hub shutdown sequence complete.")
}
