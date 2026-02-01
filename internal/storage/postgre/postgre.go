package postgre

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/MikebangSfilya/wb/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, cfg *config.Config) (*Storage, error) {
	const op = "storage.Pool.New"

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	)
	slog.Info("connecting to database",
		slog.String("op", op),
		slog.String("host", cfg.Database.Host),
		slog.String("database", cfg.Database.Name),
	)

	dbPool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := dbPool.Ping(context.Background()); err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	slog.Info("Database connected successfully!")
	return &Storage{Pool: dbPool}, nil
}

func (s *Storage) Close() {
	s.Pool.Close()
}
