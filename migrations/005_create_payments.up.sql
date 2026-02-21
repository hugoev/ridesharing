-- Create payments table
CREATE TABLE IF NOT EXISTS payments (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ride_id       UUID NOT NULL REFERENCES rides(id),
    amount        NUMERIC(10,2) NOT NULL,
    currency      VARCHAR(3)    NOT NULL DEFAULT 'USD',
    status        VARCHAR(20)   NOT NULL DEFAULT 'pending',
    stripe_txn_id VARCHAR(255),
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_ride_id ON payments (ride_id);
CREATE INDEX idx_payments_status  ON payments (status);
