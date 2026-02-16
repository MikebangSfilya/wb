package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var ErrCacheMiss = errors.New("cache miss")

type Redis struct {
	Client *redis.Client
	tr     trace.Tracer
}

func New(ctx context.Context, host, port, password string, db int) (*Redis, error) {
	const op = "repository.redis.New"

	log := slog.With("op", op)

	addr := net.JoinHostPort(host, port)

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		log.Error("failed to connect to redis",
			slog.String("op", op),
			slog.String("addr", addr),
			slog.Any("error", err),
		)
		_ = client.Close()
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	slog.Info("Redis connected successfully", slog.String("addr", addr))

	return &Redis{Client: client, tr: otel.Tracer("redis")}, nil
}

func (r *Redis) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	ctx, span := r.tr.Start(ctx, "redis.Set")
	defer span.End()
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
	}
	return r.Client.Set(ctx, key, data, ttl).Err()
}

func (r *Redis) Get(ctx context.Context, key string, dest any) error {
	ctx, span := r.tr.Start(ctx, "redis.Get")
	defer span.End()
	data, err := r.Client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return json.Unmarshal(data, dest)
}

func (r *Redis) Delete(ctx context.Context, key string) error {
	return r.Client.Del(ctx, key).Err()
}

func (r *Redis) Close() error {
	return r.Client.Close()
}
