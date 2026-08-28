package service

import (
	"context"
	"errors"
	"fmt"

	"simtelemetry-hub/internal/repository"
)

var (
	ErrEmptyDriverName = errors.New("driver_name cannot be empty")
	ErrEmptyTrackName  = errors.New("track_name cannot be empty")
	ErrInvalidLapTime  = errors.New("lap_time must be positive")
)

type TelemetryService interface {
	ProcessTelemetry(payload repository.TelemetryPayload) error
	GetLeaderboard(ctx context.Context, trackName string, limit int) ([]repository.LeaderboardEntry, error)
	GetQueueStatus() (int, int)
}

type Service struct {
	repo       repository.TelemetryRepository
	workerPool *WorkerPool
}

func NewTelemetryService(repo repository.TelemetryRepository, pool *WorkerPool) *Service {
	return &Service{
		repo:       repo,
		workerPool: pool,
	}
}

func (s *Service) ProcessTelemetry(p repository.TelemetryPayload) error {
	// Validate required fields
	if p.DriverName == "" {
		return ErrEmptyDriverName
	}
	if p.TrackName == "" {
		return ErrEmptyTrackName
	}
	if p.LapTime < 0 {
		return ErrInvalidLapTime
	}

	// Dispatch job asynchronously to the worker pool
	if err := s.workerPool.Submit(p); err != nil {
		return fmt.Errorf("failed to submit telemetry to queue: %w", err)
	}

	return nil
}

func (s *Service) GetLeaderboard(ctx context.Context, trackName string, limit int) ([]repository.LeaderboardEntry, error) {
	if trackName == "" {
		return nil, ErrEmptyTrackName
	}
	return s.repo.GetLeaderboardByTrack(ctx, trackName, limit)
}

func (s *Service) GetQueueStatus() (pending int, poolSize int) {
	return s.workerPool.QueueLength(), s.workerPool.workerCount
}
