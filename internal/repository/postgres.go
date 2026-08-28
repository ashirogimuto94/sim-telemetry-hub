package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type TelemetryRepository interface {
	SaveTelemetry(ctx context.Context, payload TelemetryPayload) error
	UpsertLeaderboard(ctx context.Context, trackName, driverName, carModel string, lapTime float64) error
	GetLeaderboardByTrack(ctx context.Context, trackName string, limit int) ([]LeaderboardEntry, error)
	Ping(ctx context.Context) error
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) SaveTelemetry(ctx context.Context, p TelemetryPayload) error {
	query := `
		INSERT INTO telemetry_logs (
			session_id, driver_name, car_model, track_name, lap_number,
			lap_time, speed, sector1_time, sector2_time, sector3_time, incident_flags
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.ExecContext(
		ctx, query,
		p.SessionID, p.DriverName, p.CarModel, p.TrackName, p.LapNumber,
		p.LapTime, p.Speed, p.Sector1Time, p.Sector2Time, p.Sector3Time, p.IncidentFlags,
	)
	if err != nil {
		return fmt.Errorf("failed to insert telemetry log: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpsertLeaderboard(ctx context.Context, trackName, driverName, carModel string, lapTime float64) error {
	query := `
		INSERT INTO leaderboard (track_name, driver_name, car_model, best_lap_time, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (track_name, driver_name)
		DO UPDATE SET
			best_lap_time = LEAST(leaderboard.best_lap_time, EXCLUDED.best_lap_time),
			car_model = CASE WHEN EXCLUDED.best_lap_time < leaderboard.best_lap_time THEN EXCLUDED.car_model ELSE leaderboard.car_model END,
			updated_at = CASE WHEN EXCLUDED.best_lap_time < leaderboard.best_lap_time THEN EXCLUDED.updated_at ELSE leaderboard.updated_at END
	`
	_, err := r.db.ExecContext(ctx, query, trackName, driverName, carModel, lapTime, time.Now())
	if err != nil {
		return fmt.Errorf("failed to upsert leaderboard: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetLeaderboardByTrack(ctx context.Context, trackName string, limit int) ([]LeaderboardEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		SELECT track_name, driver_name, car_model, best_lap_time, updated_at
		FROM leaderboard
		WHERE track_name = $1
		ORDER BY best_lap_time ASC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, trackName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	rank := 1
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(&entry.TrackName, &entry.DriverName, &entry.CarModel, &entry.BestLapTime, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan leaderboard row: %w", err)
		}
		entry.Rank = rank
		rank++
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating leaderboard rows: %w", err)
	}

	return entries, nil
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
