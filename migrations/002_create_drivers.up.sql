-- Create drivers table
CREATE TABLE IF NOT EXISTS drivers (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    vehicle_type  VARCHAR(100) NOT NULL DEFAULT '',
    license_plate VARCHAR(50)  NOT NULL DEFAULT '',
    is_available  BOOLEAN      NOT NULL DEFAULT false,
    rating        NUMERIC(3,2) NOT NULL DEFAULT 5.00,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_drivers_available ON drivers (is_available) WHERE is_available = true;
