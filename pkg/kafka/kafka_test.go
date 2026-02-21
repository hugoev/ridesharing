package kafka

import (
	"testing"
)

func TestNewProducer_Creates(t *testing.T) {
	// NewProducer doesn't connect until Publish is called
	p := NewProducer([]string{"localhost:9092"}, "test-topic")
	if p == nil {
		t.Fatal("expected non-nil producer")
	}
	if p.writer == nil {
		t.Fatal("expected non-nil writer")
	}
	// Clean up
	p.Close()
}

func TestNewConsumer_Creates(t *testing.T) {
	c := NewConsumer([]string{"localhost:9092"}, "test-topic", "test-group")
	if c == nil {
		t.Fatal("expected non-nil consumer")
	}
	if c.reader == nil {
		t.Fatal("expected non-nil reader")
	}
	// Clean up
	c.Close()
}

func TestNewProducer_MultipleBrokers(t *testing.T) {
	p := NewProducer([]string{"broker1:9092", "broker2:9092", "broker3:9092"}, "events")
	if p == nil {
		t.Fatal("expected non-nil producer")
	}
	p.Close()
}
