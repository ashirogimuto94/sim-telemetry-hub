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
	log.Println("Запуск сервиса SimTelemetry Hub...")

	// 1. Загрузка конфигурации
	cfg := config.Load()

	// 2. Инициализация пула соединений с базой данных
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
		log.Fatalf("Критическая ошибка базы данных: %v", err)
	}
	defer db.Close()
	log.Println("Подключение к PostgreSQL успешно установлено.")

	// 3. Инициализация слоя репозитория
	repo := repository.NewPostgresRepository(db)

	// 4. Инициализация пула воркеров
	workerPool := service.NewWorkerPool(cfg.WorkerPoolSize, cfg.JobQueueBuffer, repo)
	workerPool.Start()

	// 5. Инициализация сервиса и обработчиков (хендлеров)
	svc := service.NewTelemetryService(repo, workerPool)
	h := handler.NewTelemetryHandler(svc, repo)

	// 6. Настройка роутера Chi и промежуточного ПО (middleware)
	r := chi.NewRouter()
	r.Use(handler.RecoveryMiddleware)
	r.Use(handler.LoggerMiddleware)
	r.Use(handler.JSONMiddleware)

	// Регистрация маршрутов API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/telemetry", h.IngestTelemetry)
		r.Get("/leaderboard", h.GetLeaderboard)
		r.Get("/health", h.HealthCheck)
	})

	// 7. Настройка HTTP-сервера
	serverAddr := fmt.Sprintf(":%s", cfg.ServerPort)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 8. Запуск сервера в отдельной горутине
	go func() {
		log.Printf("HTTP-сервер слушает порт %s...", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка в работе HTTP-сервера: %v", err)
		}
	}()

	// 9. Перехват сигналов для плавного завершения работы (Graceful Shutdown: SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Получен сигнал завершения. Запуск плавного завершения работы...")

	// Создание контекста с таймаутом 15 секунд для ожидания завершения сервера и воркеров
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Сначала останавливаем HTTP-сервер, чтобы прекратить прием новых запросов
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Принудительная остановка HTTP-сервера: %v", err)
	} else {
		log.Println("HTTP-сервер успешно остановлен.")
	}

	// Останавливаем пул воркеров и дожидаемся завершения активных задач
	workerPool.Stop()

	log.Println("Процедура завершения SimTelemetry Hub успешно завершена.")
}
