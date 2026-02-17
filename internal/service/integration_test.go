package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"

	"github.com/MikebangSfilya/wb/internal/lib/metrics"
	"github.com/MikebangSfilya/wb/internal/lib/validator"
	"github.com/MikebangSfilya/wb/internal/model"
	"github.com/MikebangSfilya/wb/internal/repository/postgresql"
	"github.com/MikebangSfilya/wb/internal/repository/redis"
	"github.com/MikebangSfilya/wb/internal/service"
	"github.com/MikebangSfilya/wb/internal/transport/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgMod "github.com/testcontainers/testcontainers-go/modules/postgres"
	redisMod "github.com/testcontainers/testcontainers-go/modules/redis"
	"go.opentelemetry.io/otel/trace/noop"
)

func Test_Integration(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	validator.Init()
	l := slog.Default()
	tr := noop.NewTracerProvider().Tracer("test")

	postgresContainer, err := pgMod.Run(ctx, "postgres:18-alpine",
		pgMod.WithDatabase("testdb"),
		pgMod.WithUsername("user"),
		pgMod.WithPassword("password"),
		pgMod.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, postgresContainer.Terminate(ctx)) }()

	connectionString, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err)
	defer pool.Close()

	_, err = pool.Exec(ctx, initSQL)
	require.NoError(t, err)

	redisContainer, err := redisMod.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	defer func() { require.NoError(t, redisContainer.Terminate(ctx)) }()

	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	port, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	rRepo, err := redis.New(ctx, host, port.Port(), "", 0, tr)
	require.NoError(t, err)
	defer func(rRepo *redis.Redis) {
		_ = rRepo.Close()
	}(rRepo)

	svc := service.New(l, postgresql.New(pool, tr), rRepo, metrics.NewTestMetrics(), tr)

	r := chi.NewRouter()
	r.Get("/order/{id}", handlers.New(l, svc).GetOrder())
	ts := httptest.NewServer(r)
	defer ts.Close()

	fixedTime := time.Date(2021, 11, 26, 6, 22, 19, 0, time.UTC)

	order := &model.Order{
		OrderUID:          "b563feb7b2b84b6test",
		TrackNumber:       "WBILMTESTTRACK",
		Entry:             "WBIL",
		Locale:            "en",
		InternalSignature: "",
		CustomerID:        "test",
		DeliveryService:   "meest",
		Shardkey:          "9",
		SmID:              99,
		DateCreated:       fixedTime,
		OofShard:          "1",
		Delivery: model.Delivery{
			Name:    "Test Testov",
			Phone:   "+9720000000",
			Zip:     "2639809",
			City:    "Kiryat Mozkin",
			Address: "Ploshad Mira 15",
			Region:  "Kraiot",
			Email:   "test@gmail.com",
		},
		Payment: model.Payment{
			Transaction:  "b563feb7b2b84b6test",
			RequestID:    "",
			Currency:     "USD",
			Provider:     "wbpay",
			Amount:       1817,
			PaymentDt:    1637907727,
			Bank:         "alpha",
			DeliveryCost: 1500,
			GoodsTotal:   317,
			CustomFee:    0,
		},
		Items: []model.Item{
			{
				ChrtID:      9934930,
				TrackNumber: "WBILMTESTTRACK",
				Price:       453,
				Rid:         "ab4219087a764ae0btest",
				Name:        "Mascaras",
				Sale:        30,
				Size:        "0",
				TotalPrice:  317,
				NmID:        2389212,
				Brand:       "Vivienne Sabo",
				Status:      202,
			},
		},
	}

	err = svc.CreateOrder(ctx, order)
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("%s/order/%s", ts.URL, order.OrderUID))
	require.NoError(t, err)
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var retrieved model.Order
	err = json.NewDecoder(resp.Body).Decode(&retrieved)
	require.NoError(t, err)

	assert.Equal(t, order.OrderUID, retrieved.OrderUID)
	assert.Equal(t, order.Delivery.Name, retrieved.Delivery.Name)
	assert.Equal(t, order.Payment.Amount, retrieved.Payment.Amount)
	assert.True(t, order.DateCreated.Equal(retrieved.DateCreated))
}

const initSQL = `
CREATE TABLE IF NOT EXISTS orders (
    order_uid TEXT PRIMARY KEY,
    track_number TEXT NOT NULL,
    entry TEXT NOT NULL,
    locale TEXT NOT NULL,
    internal_signature TEXT,
    customer_id TEXT NOT NULL,
    delivery_service TEXT NOT NULL,
    shardkey TEXT NOT NULL,
    sm_id INTEGER NOT NULL,
    date_created TIMESTAMPTZ NOT NULL,
    oof_shard TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS delivery (
    order_uid TEXT PRIMARY KEY REFERENCES orders(order_uid) ON DELETE CASCADE,
    name TEXT NOT NULL,
    phone TEXT NOT NULL,
    zip TEXT NOT NULL,
    city TEXT NOT NULL,
    address TEXT NOT NULL,
    region TEXT NOT NULL,
    email TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS payment (
    transaction TEXT PRIMARY KEY,
    order_uid TEXT NOT NULL REFERENCES orders(order_uid) ON DELETE CASCADE,
    request_id TEXT,
    currency TEXT NOT NULL,
    provider TEXT NOT NULL,
    amount INTEGER NOT NULL,
    payment_dt BIGINT NOT NULL,
    bank TEXT NOT NULL,
    delivery_cost INTEGER NOT NULL,
    goods_total INTEGER NOT NULL,
    custom_fee INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS items (
    id SERIAL PRIMARY KEY,
    order_uid TEXT NOT NULL REFERENCES orders(order_uid) ON DELETE CASCADE,
    chrt_id INTEGER NOT NULL,
    track_number TEXT NOT NULL,
    price INTEGER NOT NULL,
    rid TEXT NOT NULL,
    name TEXT NOT NULL,
    sale INTEGER NOT NULL,
    size TEXT NOT NULL,
    total_price INTEGER NOT NULL,
    nm_id INTEGER NOT NULL,
    brand TEXT NOT NULL,
    status INTEGER NOT NULL
);
`
