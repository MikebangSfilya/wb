package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/MikebangSfilya/wb/internal/lib/validator"
	"github.com/MikebangSfilya/wb/internal/model"

	"github.com/segmentio/kafka-go"
)

const (
	baseDelay   = 1 * time.Second
	maxDelay    = 30 * time.Second
	maxAttempts = 30
)

type Service interface {
	CreateOrder(ctx context.Context, order *model.Order) error
}
type Consumer struct {
	reader  *kafka.Reader
	service Service
	l       *slog.Logger
}

func NewConsumer(l *slog.Logger, brokers []string, group, topic string, service Service) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			GroupID:  group,
			Topic:    topic,
			MinBytes: 10e3,
			MaxBytes: 10e6,
		}),
		service: service,
		l:       l,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	c.l.Info("kafka consumer started")
	defer c.l.Info("kafka consumer stopped")

	for {
		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			c.l.Error("failed to fetch message", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		c.processWithRetry(ctx, m)

		if err := c.reader.CommitMessages(ctx, m); err != nil {
			c.l.Error("failed to commit message", "error", err)
		}
	}
}

func (c *Consumer) processWithRetry(ctx context.Context, m kafka.Message) {
	var order model.Order
	if err := json.Unmarshal(m.Value, &order); err != nil {
		c.l.Error("skipping invalid json", "error", err, "offset", m.Offset)
		return
	}
	if err := validator.Validate(&order); err != nil {
		c.l.Error("skipping invalid order", "error", err)
		return
	}

	currentDelay := baseDelay
	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}

		err := c.service.CreateOrder(ctx, &order)

		if err == nil {
			return
		}

		attempt++
		c.l.Warn("failed to create order, retrying", "error", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(currentDelay):
			currentDelay *= 2
			if currentDelay > maxDelay {
				currentDelay = maxDelay
			}
			if attempt >= maxAttempts {
				c.l.Error("too many attempts, end this")
				return
			}
		}

	}

}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
