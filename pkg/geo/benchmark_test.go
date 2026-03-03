package geo

import "testing"

func BenchmarkHaversine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Haversine(40.7128, -74.0060, 34.0522, -118.2437)
	}
}

func BenchmarkEstimateFare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		EstimateFare(10.5, 1.5)
	}
}

func BenchmarkEstimateETA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		EstimateETA(15.0)
	}
}
