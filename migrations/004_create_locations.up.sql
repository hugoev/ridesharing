-- Create locations table with PostGIS support
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS locations (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    geom      GEOMETRY(Point, 4326) NOT NULL,
    lat       DOUBLE PRECISION NOT NULL,
    long      DOUBLE PRECISION NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Spatial index for geospatial queries
CREATE INDEX idx_locations_geom    ON locations USING GIST (geom);
CREATE INDEX idx_locations_user_id ON locations (user_id);
CREATE INDEX idx_locations_time    ON locations (timestamp DESC);

-- Function to find nearby drivers within a radius (meters)
CREATE OR REPLACE FUNCTION find_nearby_drivers(
    search_lat DOUBLE PRECISION,
    search_long DOUBLE PRECISION,
    radius_meters DOUBLE PRECISION DEFAULT 5000
)
RETURNS TABLE (
    driver_id UUID,
    driver_lat DOUBLE PRECISION,
    driver_long DOUBLE PRECISION,
    distance_meters DOUBLE PRECISION
) AS $$
BEGIN
    RETURN QUERY
    SELECT DISTINCT ON (l.user_id)
        l.user_id,
        l.lat,
        l.long,
        ST_Distance(
            l.geom::geography,
            ST_SetSRID(ST_MakePoint(search_long, search_lat), 4326)::geography
        ) AS distance_meters
    FROM locations l
    INNER JOIN drivers d ON d.user_id = l.user_id
    WHERE d.is_available = true
      AND ST_DWithin(
          l.geom::geography,
          ST_SetSRID(ST_MakePoint(search_long, search_lat), 4326)::geography,
          radius_meters
      )
    ORDER BY l.user_id, l.timestamp DESC, distance_meters ASC;
END;
$$ LANGUAGE plpgsql;
