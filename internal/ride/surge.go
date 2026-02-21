package ride

import (
	"sync"

	"github.com/rs/zerolog/log"
)

// SurgeEngine calculates surge pricing multipliers based on demand-to-supply ratio.
type SurgeEngine struct {
	mu sync.RWMutex

	// Configuration
	baseThreshold int // active requests below this → no surge
	highThreshold int // active requests above this → max surge
	minMultiplier float64
	maxMultiplier float64
}

// NewSurgeEngine creates a surge engine with configurable thresholds.
func NewSurgeEngine() *SurgeEngine {
	return &SurgeEngine{
		baseThreshold: 5,
		highThreshold: 20,
		minMultiplier: 1.0,
		maxMultiplier: 3.0,
	}
}

// Calculate returns the surge multiplier based on active ride requests and available drivers.
// Formula: if demand/supply ratio exceeds threshold, multiplier scales linearly.
func (s *SurgeEngine) Calculate(activeRequests, availableDrivers int) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if activeRequests <= s.baseThreshold || availableDrivers == 0 {
		// No surge if demand is low
		if availableDrivers == 0 && activeRequests > 0 {
			// No drivers available at all — max surge
			log.Warn().Int("requests", activeRequests).Msg("no drivers available, max surge")
			return s.maxMultiplier
		}
		return s.minMultiplier
	}

	// Calculate demand/supply ratio
	ratio := float64(activeRequests) / float64(availableDrivers)

	// Linear interpolation between thresholds
	if ratio <= 1.0 {
		return s.minMultiplier
	}

	// Scale: ratio 1→3 maps to multiplier 1.0→maxMultiplier
	multiplier := s.minMultiplier + (ratio-1.0)*(s.maxMultiplier-s.minMultiplier)/2.0
	if multiplier > s.maxMultiplier {
		multiplier = s.maxMultiplier
	}

	// Round to nearest 0.1
	return float64(int(multiplier*10)) / 10
}
