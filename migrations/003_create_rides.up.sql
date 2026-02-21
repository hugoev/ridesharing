-- Create rides table
CREATE TYPE ride_status AS ENUM ('requested', 'matched', 'enroute', 'completed', 'cancelled');

CREATE TABLE IF NOT EXISTS rides (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rider_id         UUID        NOT NULL REFERENCES users(id),
    driver_id        UUID                 REFERENCES users(id),
    pickup_lat       DOUBLE PRECISION NOT NULL,
    pickup_long      DOUBLE PRECISION NOT NULL,
    dropoff_lat      DOUBLE PRECISION NOT NULL,
    dropoff_long     DOUBLE PRECISION NOT NULL,
    status           ride_status NOT NULL DEFAULT 'requested',
    fare             NUMERIC(10,2) NOT NULL DEFAULT 0,
    surge_multiplier NUMERIC(4,2) NOT NULL DEFAULT 1.00,
    estimated_eta    INTEGER,  -- seconds
    distance_km      NUMERIC(8,2),
    started_at       TIMESTAMPTZ,
    ended_at         TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rides_status    ON rides (status);
CREATE INDEX idx_rides_rider_id  ON rides (rider_id);
CREATE INDEX idx_rides_driver_id ON rides (driver_id);
CREATE INDEX idx_rides_active    ON rides (status) WHERE status IN ('requested', 'matched', 'enroute');
