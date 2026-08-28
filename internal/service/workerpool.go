package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"simtelemetry-hub/internal/repository"
)

// FastLapKey identifies driver performance per track in-memory
type FastLapKey struct {
	TrackName  string
	DriverName string
}

// In-memory cache for ultra-fast leaderboard updates using RWMutex
type MemoryCache struct {
	mu   sync.RWMutex
	laps map[FastLapKey]float64
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		laps: make(map[FastLapKey]float64),
	}
}

// UpdateIfFaster updates the cached lap time if the new lap time is faster (or if no previous entry exists).
// Returns true if a new record was set.
func (c *MemoryCache) UpdateIfFaster(track, driver string, lapTime float64) bool {
	if lapTime <= 0 {
		return false
	}
	key := FastLapKey{TrackName: track, DriverName: driver}

	// Double-checked locking pattern for high throughput
	c.mu.RLock()
	currentBest, exists := c.laps[key]
	c.mu.RUnlock()

	if exists && lapTime >= currentBest {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	currentBest, exists = c.laps[key]
	if !exists || lapTime < currentBest {
		c.laps[key] = lapTime
		return true
	}

	return false
}

// GetBestLap retrieves the cached best lap time for a track and driver
func (c *MemoryCache) GetBestLap(track, driver string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.laps[FastLapKey{TrackName: track, DriverName: driver}]
	return val, ok
}

// WorkerPool manages parallel background ingestion of telemetry events
type WorkerPool struct {
	workerCount int
	jobs        chan repository.TelemetryPayload
	repo        repository.TelemetryRepository
	cache       *MemoryCache
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

func NewWorkerPool(workerCount, bufferSize int, repo repository.TelemetryRepository) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workerCount: workerCount,
		jobs:        make(chan repository.TelemetryPayload, bufferSize),
		repo:        repo,
		cache:       NewMemoryCache(),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start launches worker goroutines
func (wp *WorkerPool) Start() {
	log.Printf("[WorkerPool] Starting %d worker goroutines...", wp.workerCount)
	for i := 1; i <= wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker is the main consumer loop for telemetry payloads
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	log.Printf("[WorkerPool] Worker #%d started", id)

	for {
		select {
		case payload, ok := <-wp.jobs:
			if !ok {
				log.Printf("[WorkerPool] Worker #%d stopping (job channel closed)", id)
				return
			}
			wp.processTelemetry(id, payload)
		case <-wp.ctx.Done():
			// Process any remaining buffered jobs before exiting
			for payload := range wp.jobs {
				wp.processTelemetry(id, payload)
			}
			log.Printf("[WorkerPool] Worker #%d stopping (context cancelled)", id)
			return
		}
	}
}

// processTelemetry handles in-memory lap calculation and DB persistence
func (wp *WorkerPool) processTelemetry(workerID int, p repository.TelemetryPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Thread-safe in-memory cache update
	isNewRecord := wp.cache.UpdateIfFaster(p.TrackName, p.DriverName, p.LapTime)
	if isNewRecord {
		log.Printf("[Worker #%d] New record! Driver: %s, Track: %s, Lap: %.3fs", workerID, p.DriverName, p.TrackName, p.LapTime)
	}

	// 2. Persist raw telemetry event to DB
	if err := wp.repo.SaveTelemetry(ctx, p); err != nil {
		log.Printf("[Worker #%d] Error saving telemetry for session %s: %v", workerID, p.SessionID, err)
	}

	// 3. If lap is valid and updated, persist to DB leaderboard
	if p.LapTime > 0 {
		if err := wp.repo.UpsertLeaderboard(ctx, p.TrackName, p.DriverName, p.CarModel, p.LapTime); err != nil {
			log.Printf("[Worker #%d] Error upserting leaderboard for %s: %v", workerID, p.DriverName, err)
		}
	}
}

// Submit queues a new telemetry job. Returns error if pool queue is full or stopped.
func (wp *WorkerPool) Submit(payload repository.TelemetryPayload) error {
	select {
	case wp.jobs <- payload:
		return nil
	default:
		return fmt.Errorf("telemetry job queue is full")
	}
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	log.Println("[WorkerPool] Initiating graceful shutdown...")
	close(wp.jobs) // Stop accepting new incoming channel submissions
	wp.cancel()    // Signal workers
	wp.wg.Wait()   // Wait for active workers to complete
	log.Println("[WorkerPool] All workers stopped cleanly.")
}

// QueueLength returns current pending jobs count
func (wp *WorkerPool) QueueLength() int {
	return len(wp.jobs)
}
