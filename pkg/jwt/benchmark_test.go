package jwtutil

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func BenchmarkGenerateToken(b *testing.B) {
	uid := uuid.New()
	for i := 0; i < b.N; i++ {
		GenerateToken(uid, "rider", "bench-secret-key-32chars!!", 1*time.Hour)
	}
}

func BenchmarkValidateToken(b *testing.B) {
	uid := uuid.New()
	token, _ := GenerateToken(uid, "rider", "bench-secret-key-32chars!!", 1*time.Hour)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateToken(token, "bench-secret-key-32chars!!")
	}
}

func BenchmarkGenerateTokenPair(b *testing.B) {
	uid := uuid.New()
	for i := 0; i < b.N; i++ {
		GenerateTokenPair(uid, "rider", "bench-secret-key-32chars!!", 1*time.Hour)
	}
}
