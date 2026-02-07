package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/MikebangSfilya/wb/internal/model"
	"github.com/MikebangSfilya/wb/internal/repository/redis"
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
}

func New(l *slog.Logger, repo Repository, cache Cache) *OrderService {
	return &OrderService{
		repo:  repo,
		cache: cache,
		l:     l,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, order *model.Order) error {
	const op = "service.CreateOrder"
	if err := s.repo.CreateOrder(ctx, order); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := s.cache.Set(ctx, order.OrderUID, order, 24*time.Hour); err != nil {
		s.l.Error("failed to cache order", "error", err, "uid", order.OrderUID)
	}
	return nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderUID string) (*model.Order, error) {
	const op = "service.GetOrder"
	var order model.Order

	err := s.cache.Get(ctx, orderUID, &order)
	if err == nil {
		s.l.Debug("got order", "uid", orderUID)
		return &order, nil
	}

	if !errors.Is(err, redis.ErrCacheMiss) {
		s.l.Error("service: cache error", "error", err)
	}

	orderPtr, err := s.repo.GetOrder(ctx, orderUID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if err := s.cache.Set(ctx, orderPtr.OrderUID, orderPtr, 24*time.Hour); err != nil {
		s.l.Error("failed to cache order", "error", err, "uid", orderPtr.OrderUID)
	}

	return orderPtr, nil
}
