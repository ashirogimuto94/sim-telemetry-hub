-- Telemetry events table
CREATE TABLE IF NOT EXISTS telemetry_logs (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL,
    driver_name VARCHAR(100) NOT NULL,
    car_model VARCHAR(100) NOT NULL,
    track_name VARCHAR(100) NOT NULL,
    lap_number INT NOT NULL,
    lap_time NUMERIC(8, 3) NOT NULL,
    speed NUMERIC(6, 2) NOT NULL,
    sector1_time NUMERIC(8, 3) DEFAULT 0.0,
    sector2_time NUMERIC(8, 3) DEFAULT 0.0,
    sector3_time NUMERIC(8, 3) DEFAULT 0.0,
    incident_flags INT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Leaderboard table (aggregated best lap per track & driver)
CREATE TABLE IF NOT EXISTS leaderboard (
    id SERIAL PRIMARY KEY,
    track_name VARCHAR(100) NOT NULL,
    driver_name VARCHAR(100) NOT NULL,
    car_model VARCHAR(100) NOT NULL,
    best_lap_time NUMERIC(8, 3) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_track_driver UNIQUE (track_name, driver_name)
);

-- Indices for rapid querying
CREATE INDEX IF NOT EXISTS idx_telemetry_track ON telemetry_logs (track_name);
CREATE INDEX IF NOT EXISTS idx_telemetry_driver ON telemetry_logs (driver_name);
CREATE INDEX IF NOT EXISTS idx_leaderboard_track_lap ON leaderboard (track_name, best_lap_time ASC);
