package ride

import (
	"testing"

	"github.com/hugovillarreal/ridesharing/internal/models"
)

func TestValidTransitions(t *testing.T) {
	valid := []struct {
		from, to models.RideStatus
	}{
		{models.RideStatusRequested, models.RideStatusMatched},
		{models.RideStatusRequested, models.RideStatusCancelled},
		{models.RideStatusMatched, models.RideStatusEnroute},
		{models.RideStatusMatched, models.RideStatusCancelled},
		{models.RideStatusEnroute, models.RideStatusCompleted},
		{models.RideStatusEnroute, models.RideStatusCancelled},
	}

	for _, tt := range valid {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err != nil {
				t.Errorf("expected valid transition from %s to %s, got error: %v", tt.from, tt.to, err)
			}
		})
	}
}

func TestInvalidTransitions(t *testing.T) {
	invalid := []struct {
		from, to models.RideStatus
	}{
		{models.RideStatusCompleted, models.RideStatusRequested},
		{models.RideStatusCompleted, models.RideStatusMatched},
		{models.RideStatusCancelled, models.RideStatusRequested},
		{models.RideStatusRequested, models.RideStatusEnroute},
		{models.RideStatusRequested, models.RideStatusCompleted},
		{models.RideStatusMatched, models.RideStatusCompleted},
		{models.RideStatusMatched, models.RideStatusRequested},
		{models.RideStatusEnroute, models.RideStatusRequested},
		{models.RideStatusEnroute, models.RideStatusMatched},
	}

	for _, tt := range invalid {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err == nil {
				t.Errorf("expected invalid transition from %s to %s, but got no error", tt.from, tt.to)
			}
		})
	}
}
