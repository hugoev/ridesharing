package logger

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestInit_ValidLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "fatal"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			Init(level) // should not panic
			current := zerolog.GlobalLevel()
			expected, _ := zerolog.ParseLevel(level)
			if current != expected {
				t.Errorf("expected level %s, got %s", expected, current)
			}
		})
	}
}

func TestInit_InvalidLevel(t *testing.T) {
	Init("garbage") // should fallback to InfoLevel
	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Errorf("expected fallback to info level, got %s", zerolog.GlobalLevel())
	}
}

func TestInit_EmptyLevel(t *testing.T) {
	// zerolog.ParseLevel("") returns NoLevel (not an error),
	// so Init("") just sets global to NoLevel.
	// We just verify it doesn't panic.
	Init("")
}
