package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/MikebangSfilya/wb/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) CreateOrder(ctx context.Context, order *model.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockRepo) GetOrder(ctx context.Context, orderUID string) (*model.Order, error) {
	args := m.Called(ctx, orderUID)
	return args.Get(0).(*model.Order), args.Error(1)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCache) Get(ctx context.Context, key string, dest any) error {
	args := m.Called(ctx, key, dest)
	return args.Error(0)
}

func TestOrderService_CreateOrder(t *testing.T) {
	type mockBehavior func(r *MockRepo, c *MockCache, order *model.Order)

	tests := []struct {
		name         string
		order        *model.Order
		mockBehavior mockBehavior
		wantErr      bool
	}{
		{
			name:  "success",
			order: &model.Order{OrderUID: "034"},
			mockBehavior: func(r *MockRepo, c *MockCache, order *model.Order) {
				r.On("CreateOrder", mock.Anything, order).Return(nil)
				c.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "db_error",
			order: &model.Order{OrderUID: "034"},
			mockBehavior: func(r *MockRepo, c *MockCache, order *model.Order) {
				r.On("CreateOrder", mock.Anything, order).Return(errors.New("db error"))
			},
			wantErr: true,
		},
		{
			name:  "cache_error",
			order: &model.Order{OrderUID: "034"},
			mockBehavior: func(r *MockRepo, c *MockCache, order *model.Order) {
				r.On("CreateOrder", mock.Anything, order).Return(nil)
				c.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("cache error"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockRepo := &MockRepo{}
			mockCache := &MockCache{}
			tt.mockBehavior(mockRepo, mockCache, tt.order)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			svc := New(logger, mockRepo, mockCache)
			err := svc.CreateOrder(context.Background(), tt.order)
			if !tt.wantErr {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			mockRepo.AssertExpectations(t)
			mockCache.AssertExpectations(t)
		})
	}
}

func TestOrderService_GetOrder(t *testing.T) {
	type mockBehavior func(r *MockRepo, c *MockCache, order *model.Order)
	tests := []struct {
		name         string
		order        *model.Order
		mockBehavior mockBehavior
		wantErr      bool
	}{
		{
			name:  "success_cache",
			order: &model.Order{OrderUID: "034"},
			mockBehavior: func(r *MockRepo, c *MockCache, order *model.Order) {
				c.On("Get", mock.Anything, order.OrderUID, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "cache_miss",
			order: &model.Order{OrderUID: "034"},
			mockBehavior: func(r *MockRepo, c *MockCache, order *model.Order) {
				c.On("Get", mock.Anything, order.OrderUID, mock.Anything).Return(errors.New("cache miss"))
				r.On("GetOrder", mock.Anything, order.OrderUID).Return(order, nil)
				c.On("Set", mock.Anything, order.OrderUID, order, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockRepo := &MockRepo{}
			mockCache := &MockCache{}
			tt.mockBehavior(mockRepo, mockCache, tt.order)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			svc := New(logger, mockRepo, mockCache)
			order, err := svc.GetOrder(context.Background(), tt.order.OrderUID)
			if !tt.wantErr {
				assert.NoError(t, err)
				assert.NotNil(t, order)
			}
		})
	}
}
