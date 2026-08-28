package repository

import "time"

// TelemetryPayload represents incoming raw JSON from race simulators
type TelemetryPayload struct {
	SessionID     string  `json:"session_id"`
	DriverName    string  `json:"driver_name"`
	CarModel      string  `json:"car_model"`
	TrackName     string  `json:"track_name"`
	LapNumber     int     `json:"lap_number"`
	LapTime       float64 `json:"lap_time"`
	Speed         float64 `json:"speed"`
	Sector1Time   float64 `json:"sector1_time"`
	Sector2Time   float64 `json:"sector2_time"`
	Sector3Time   float64 `json:"sector3_time"`
	IncidentFlags int     `json:"incident_flags"`
}

// TelemetryRecord represents stored DB telemetry log entry
type TelemetryRecord struct {
	ID            int       `json:"id"`
	SessionID     string    `json:"session_id"`
	DriverName    string    `json:"driver_name"`
	CarModel      string    `json:"car_model"`
	TrackName     string    `json:"track_name"`
	LapNumber     int       `json:"lap_number"`
	LapTime       float64   `json:"lap_time"`
	Speed         float64   `json:"speed"`
	Sector1Time   float64   `json:"sector1_time"`
	Sector2Time   float64   `json:"sector2_time"`
	Sector3Time   float64   `json:"sector3_time"`
	IncidentFlags int       `json:"incident_flags"`
	CreatedAt     time.Time `json:"created_at"`
}

// LeaderboardEntry represents top lap times per track
type LeaderboardEntry struct {
	Rank         int       `json:"rank,omitempty"`
	TrackName    string    `json:"track_name"`
	DriverName   string    `json:"driver_name"`
	CarModel     string    `json:"car_model"`
	BestLapTime  float64   `json:"best_lap_time"`
	UpdatedAt    time.Time `json:"updated_at"`
}
