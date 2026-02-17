package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/MikebangSfilya/wb/internal/lib/metrics"
	"github.com/MikebangSfilya/wb/internal/model"
	"github.com/MikebangSfilya/wb/internal/repository/redis"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Repository interface {
	CreateOrder(ctx context.Context, order *model.Order) error
	GetOrder(ctx context.Context, orderUID string) (*model.Order, error)
}

type Cache interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string, dest any) error
}

type OrderService struct {
	repo  Repository
	cache Cache
	l     *slog.Logger
	tr    trace.Tracer
	m     *metrics.Metrics
}

func New(l *slog.Logger, repo Repository, cache Cache, m *metrics.Metrics, tr trace.Tracer) *OrderService {
	return &OrderService{
		repo:  repo,
		cache: cache,
		l:     l,
		tr:    tr,
		m:     m,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, order *model.Order) error {
	const op = "service.CreateOrder"
	ctx, span := s.tr.Start(ctx, "service.CreateOrder")
	defer span.End()
	span.SetAttributes(attribute.String("order_uid", order.OrderUID))
	if err := s.repo.CreateOrder(ctx, order); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("%s: %w", op, err)
	}
	s.m.OrdersCreated.Inc()
	s.setCache(ctx, order.OrderUID, order, 24*time.Hour)
	return nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderUID string) (*model.Order, error) {
	const op = "service.GetOrder"

	ctx, span := s.tr.Start(ctx, "service.GetOrder")
	defer span.End()
	span.SetAttributes(attribute.String("uid", orderUID))

	var order model.Order

	cacheCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	err := s.cache.Get(cacheCtx, orderUID, &order)
	if err == nil {
		s.m.CacheHits.Inc()
		s.l.Debug("got order", "uid", orderUID)
		return &order, nil
	}

	s.m.CacheMisses.Inc()

	if !errors.Is(err, redis.ErrCacheMiss) {
		s.l.Error("service: cache error", "error", err)
	}

	orderPtr, err := s.repo.GetOrder(ctx, orderUID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	s.setCache(ctx, orderPtr.OrderUID, orderPtr, 24*time.Hour)

	return orderPtr, nil
}

func (s *OrderService) setCache(ctx context.Context, key string, value any, ttl time.Duration) {
	cacheCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	if err := s.cache.Set(cacheCtx, key, value, ttl); err != nil {

		s.l.Error("async cache set failed",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
	}
}
