package geo

import (
	"math"
	"testing"
	"time"
)

func TestHaversine(t *testing.T) {
	tests := []struct {
		name     string
		lat1     float64
		lon1     float64
		lat2     float64
		lon2     float64
		expected float64 // km, approximate
		epsilon  float64 // tolerance
	}{
		{
			name: "New York to Los Angeles",
			lat1: 40.7128, lon1: -74.0060,
			lat2: 34.0522, lon2: -118.2437,
			expected: 3944.0,
			epsilon:  10.0,
		},
		{
			name: "Same point",
			lat1: 51.5074, lon1: -0.1278,
			lat2: 51.5074, lon2: -0.1278,
			expected: 0.0,
			epsilon:  0.001,
		},
		{
			name: "Short distance - about 1km",
			lat1: 40.7128, lon1: -74.0060,
			lat2: 40.7218, lon2: -74.0060,
			expected: 1.0,
			epsilon:  0.1,
		},
		{
			name: "London to Paris",
			lat1: 51.5074, lon1: -0.1278,
			lat2: 48.8566, lon2: 2.3522,
			expected: 343.5,
			epsilon:  5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Haversine(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(result-tt.expected) > tt.epsilon {
				t.Errorf("Haversine(%v, %v, %v, %v) = %v km, want ~%v km (±%v)",
					tt.lat1, tt.lon1, tt.lat2, tt.lon2, result, tt.expected, tt.epsilon)
			}
		})
	}
}

func TestEstimateETA(t *testing.T) {
	// 10km at 35km/h with 1.3 road factor ≈ 22.3 minutes
	eta := EstimateETA(10.0)
	minutes := eta.Minutes()
	if minutes < 20 || minutes > 25 {
		t.Errorf("EstimateETA(10km) = %v minutes, want ~22 minutes", minutes)
	}

	// 0km should be 0
	eta = EstimateETA(0)
	if eta != 0 {
		t.Errorf("EstimateETA(0km) = %v, want 0", eta)
	}
}

func TestEstimateFare(t *testing.T) {
	// 10km, no surge: 2.50 + (10 * 1.50 * 1.0) + 1.00 = 18.50
	fare := EstimateFare(10.0, 1.0)
	if fare != 18.50 {
		t.Errorf("EstimateFare(10km, 1.0x) = %v, want 18.50", fare)
	}

	// 10km, 2x surge: 2.50 + (10 * 1.50 * 2.0) + 1.00 = 33.50
	fare = EstimateFare(10.0, 2.0)
	if fare != 33.50 {
		t.Errorf("EstimateFare(10km, 2.0x) = %v, want 33.50", fare)
	}

	// 0km: just base + booking = 3.50
	fare = EstimateFare(0, 1.0)
	if fare != 3.50 {
		t.Errorf("EstimateFare(0km, 1.0x) = %v, want 3.50", fare)
	}
}

func TestEstimateETAReturnsPositiveDuration(t *testing.T) {
	eta := EstimateETA(5.0)
	if eta <= 0 {
		t.Errorf("EstimateETA(5km) should be positive, got %v", eta)
	}
	if eta > time.Hour {
		t.Errorf("EstimateETA(5km) should be less than an hour, got %v", eta)
	}
}
