// Package kafka provides Kafka producer and consumer helpers.
package kafka

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	kafkago "github.com/segmentio/kafka-go"
)

// Producer wraps a kafka-go writer for publishing messages.
type Producer struct {
	writer *kafkago.Writer
}

// NewProducer creates a new Kafka producer for the given topic.
func NewProducer(brokers []string, topic string) *Producer {
	w := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafkago.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		RequiredAcks: kafkago.RequireOne,
	}
	return &Producer{writer: w}
}

// Publish sends a message with the given key and value.
func (p *Producer) Publish(ctx context.Context, key, value []byte) error {
	return p.writer.WriteMessages(ctx, kafkago.Message{
		Key:   key,
		Value: value,
	})
}

// Close shuts down the producer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// Consumer wraps a kafka-go reader for consuming messages.
type Consumer struct {
	reader *kafkago.Reader
}

// NewConsumer creates a new Kafka consumer for the given topic and group.
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
		StartOffset:    kafkago.LastOffset,
	})
	return &Consumer{reader: r}
}

// Consume reads messages in a loop and calls the handler for each one.
// It blocks until the context is cancelled.
func (c *Consumer) Consume(ctx context.Context, handler func(key, value []byte) error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := c.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Error().Err(err).Str("topic", c.reader.Config().Topic).Msg("kafka read error")
				time.Sleep(time.Second)
				continue
			}

			if err := handler(msg.Key, msg.Value); err != nil {
				log.Error().Err(err).
					Str("topic", c.reader.Config().Topic).
					Str("key", string(msg.Key)).
					Msg("kafka message handler error")
			}
		}
	}
}

// Close shuts down the consumer.
func (c *Consumer) Close() error {
	return c.reader.Close()
}
