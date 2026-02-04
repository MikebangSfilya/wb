package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(ctx context.Context, brokers []string, topic string) (*Producer, error) {
	const op = "kafka.NewProducer"
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	err = conn.Close()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	w := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.LeastBytes{},
		WriteTimeout:           5 * time.Second,
		RequiredAcks:           kafka.RequireOne,
		AllowAutoTopicCreation: true,
	}
	return &Producer{writer: w}, nil
}

func (p *Producer) SendMessage(ctx context.Context, key string, value []byte) error {
	const op = "kafka.Producer.SendMessage"

	msg := kafka.Message{
		Key:   []byte(key),
		Value: value,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
