package postgre

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/MikebangSfilya/wb/internal/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
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

	dbConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	dbConfig.MaxConns = 10
	dbConfig.MinConns = 2
	dbConfig.MaxConnLifetime = time.Hour

	dbPool, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := dbPool.Ping(ctx); err != nil {
		dbPool.Close()
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	slog.Info("Database connected successfully!")
	return &Storage{Pool: dbPool}, nil
}

func (s *Storage) Close() {
	s.Pool.Close()
}

func RunMigrations(cfg *config.Config) error {
	const op = "storage.postgre.RunMigrations"

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	)

	m, err := migrate.New("file://db/migrations", connStr)
	if err != nil {
		return fmt.Errorf("%s: failed to create migrate instance: %w", op, err)
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("migrations: no changes", slog.String("op", op))
			return nil
		}
		return fmt.Errorf("%s: failed to apply migrations: %w", op, err)
	}

	slog.Info("migrations applied successfully", slog.String("op", op))
	return nil
}
