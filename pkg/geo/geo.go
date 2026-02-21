// Package geo provides geospatial utility functions.
package geo

import (
	"math"
	"time"
)

const (
	// EarthRadiusKm is the mean radius of the Earth in kilometers.
	EarthRadiusKm = 6371.0

	// AverageSpeedKmH is the assumed average vehicle speed for ETA estimation.
	AverageSpeedKmH = 35.0
)

// Haversine calculates the great-circle distance in kilometers between two
// points specified by their latitude and longitude in decimal degrees.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := degreesToRadians(lat1)
	lat2Rad := degreesToRadians(lat2)
	deltaLat := degreesToRadians(lat2 - lat1)
	deltaLon := degreesToRadians(lon2 - lon1)

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusKm * c
}

// EstimateETA returns an estimated travel time for the given distance.
// It applies a 1.3x road-factor multiplier to account for non-straight routes.
func EstimateETA(distanceKm float64) time.Duration {
	roadFactor := 1.3
	hours := (distanceKm * roadFactor) / AverageSpeedKmH
	return time.Duration(hours * float64(time.Hour))
}

// EstimateFare calculates a fare based on distance, surge multiplier, and rate.
func EstimateFare(distanceKm, surgeMultiplier float64) float64 {
	baseFare := 2.50   // base charge in dollars
	perKmRate := 1.50  // per-kilometer rate
	bookingFee := 1.00 // flat booking fee

	fare := baseFare + (distanceKm * perKmRate * surgeMultiplier) + bookingFee
	return math.Round(fare*100) / 100 // round to 2 decimal places
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
