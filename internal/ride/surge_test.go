package ride

import (
	"testing"
)

func TestSurgeEngine_NoSurge(t *testing.T) {
	s := NewSurgeEngine()

	// Low demand, plenty of drivers → no surge
	m := s.Calculate(3, 10) // 3 requests, 10 drivers
	if m != 1.0 {
		t.Errorf("Calculate(3, 10) = %v, want 1.0", m)
	}
}

func TestSurgeEngine_NoDrivers(t *testing.T) {
	s := NewSurgeEngine()

	// Requests exist but no drivers → max surge
	m := s.Calculate(5, 0)
	if m != 3.0 {
		t.Errorf("Calculate(5, 0) = %v, want 3.0 (max)", m)
	}
}

func TestSurgeEngine_ZeroDemandZeroSupply(t *testing.T) {
	s := NewSurgeEngine()

	// No requests, no drivers → 1.0
	m := s.Calculate(0, 0)
	if m != 1.0 {
		t.Errorf("Calculate(0, 0) = %v, want 1.0", m)
	}
}

func TestSurgeEngine_HighDemandLowSupply(t *testing.T) {
	s := NewSurgeEngine()

	// 15 requests, 5 drivers → ratio 3.0 → high surge
	m := s.Calculate(15, 5)
	if m <= 1.0 {
		t.Errorf("Calculate(15, 5) = %v, want > 1.0", m)
	}
	if m > 3.0 {
		t.Errorf("Calculate(15, 5) = %v, want <= 3.0 (max)", m)
	}
}

func TestSurgeEngine_BalancedDemandSupply(t *testing.T) {
	s := NewSurgeEngine()

	// 10 requests, 10 drivers → ratio 1.0 → no surge
	m := s.Calculate(10, 10)
	if m != 1.0 {
		t.Errorf("Calculate(10, 10) = %v, want 1.0", m)
	}
}

func TestSurgeEngine_BelowThreshold(t *testing.T) {
	s := NewSurgeEngine()

	// Below base threshold (5) → no surge regardless
	m := s.Calculate(4, 2)
	if m != 1.0 {
		t.Errorf("Calculate(4, 2) = %v, want 1.0 (below threshold)", m)
	}
}

func TestSurgeEngine_CapsAtMax(t *testing.T) {
	s := NewSurgeEngine()

	// Extreme demand → should cap at max multiplier
	m := s.Calculate(100, 1)
	if m != 3.0 {
		t.Errorf("Calculate(100, 1) = %v, want 3.0 (capped)", m)
	}
}

func TestSurgeEngine_Monotonic(t *testing.T) {
	s := NewSurgeEngine()

	// Surge should increase (or stay same) as demand increases
	drivers := 5
	prev := s.Calculate(5, drivers)
	for requests := 6; requests <= 30; requests++ {
		m := s.Calculate(requests, drivers)
		if m < prev {
			t.Errorf("surge decreased from %v (requests=%d) to %v (requests=%d)",
				prev, requests-1, m, requests)
		}
		prev = m
	}
}
