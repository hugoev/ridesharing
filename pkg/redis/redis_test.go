package redis

import (
	"testing"
	"time"
)

func TestNewDistLock_KeyPrefix(t *testing.T) {
	// Can't test with real Redis, but verify struct creation
	lock := NewDistLock(nil, "test-resource", 5*time.Second)

	if lock.key != "lock:test-resource" {
		t.Errorf("expected key 'lock:test-resource', got %q", lock.key)
	}
	if lock.ttl != 5*time.Second {
		t.Errorf("expected ttl 5s, got %v", lock.ttl)
	}
	if lock.value == "" {
		t.Error("expected non-empty unique lock value")
	}
}

func TestNewDistLock_UniqueValues(t *testing.T) {
	lock1 := NewDistLock(nil, "resource", time.Second)
	lock2 := NewDistLock(nil, "resource", time.Second)

	if lock1.value == lock2.value {
		t.Error("expected unique lock values for different instances")
	}
}

func TestNewDistLock_DifferentKeys(t *testing.T) {
	lock1 := NewDistLock(nil, "resource-a", time.Second)
	lock2 := NewDistLock(nil, "resource-b", time.Second)

	if lock1.key == lock2.key {
		t.Error("expected different keys for different resources")
	}
}
