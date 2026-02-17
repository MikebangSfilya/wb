package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MikebangSfilya/wb/internal/config"
	sl2 "github.com/MikebangSfilya/wb/internal/lib/log"
	"github.com/MikebangSfilya/wb/internal/lib/metrics"
	"github.com/MikebangSfilya/wb/internal/lib/tracing"
	"github.com/MikebangSfilya/wb/internal/lib/validator"
	"github.com/MikebangSfilya/wb/internal/repository/postgresql"
	redis2 "github.com/MikebangSfilya/wb/internal/repository/redis"
	"github.com/MikebangSfilya/wb/internal/service"
	"github.com/MikebangSfilya/wb/internal/storage/postgre"
	"github.com/MikebangSfilya/wb/internal/transport/handlers"
	"github.com/MikebangSfilya/wb/internal/transport/kafka"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riandyrn/otelchi"
	"golang.org/x/sync/errgroup"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	validator.Init()

	sl := sl2.SetupLogger(cfg.Env)
	slog.SetDefault(sl)
	sl.Info("config loaded, start application")

	m := metrics.New()

	shutdownTracer, err := tracing.InitTracer(context.Background(), "wb-service", cfg.Otel.Address)
	if err != nil {
		sl.Error("failed to init tracer", "error", err)
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := shutdownTracer(ctx); err != nil {
				sl.Error("failed to shutdown tracer", "error", err)
			}
		}()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgre.New(ctx, cfg)
	if err != nil {
		sl.Error("Database connection failed", "error", err)
		os.Exit(1)
	}

	if err := postgre.RunMigrations(cfg); err != nil {
		sl.Error("Database migrations failed, use make migrate-up", "error", err)
	}

	r, err := redis2.New(ctx, cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		sl.Error("Redis connection failed", "error", err)
		os.Exit(1)
	}
	repo := postgresql.New(db.Pool)
	svc := service.New(sl, repo, r, m)

	consumer := kafka.NewConsumer(sl, cfg.Kafka.Brokers, cfg.Kafka.GroupID, cfg.Kafka.Topic, svc)

	h := handlers.New(sl, svc)

	router := chi.NewRouter()
	router.Use(otelchi.Middleware("wb-service", otelchi.WithChiRoutes(router)))
	router.Use(m.Middleware)
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Handle("/metrics", promhttp.Handler())
	router.Get("/order/{id}", h.GetOrder())
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/static/index.html")
	})
	srv := newServer(cfg, router)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	sl.Info("Server is running",
		slog.String("url", "http://"+cfg.HTTPServer.Address),
	)

	g.Go(func() error {
		return consumer.Start(ctx)
	})

	g.Go(func() error {
		<-ctx.Done()
		sl.Info("shutting down gracefully...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			sl.Error("Server forced to shutdown", "error", err)
			return err
		}
		db.Close()

		if err := consumer.Close(); err != nil {
			sl.Error("Kafka consumer close error", "error", err)
		}

		if err := r.Close(); err != nil {
			sl.Error("Server forced to close", "error", err)
		}

		sl.Info("shutting down gracefully")
		return nil
	})
	if err := g.Wait(); err != nil {
		sl.Error("Application exit with error", "error", err)
	}
}

func newServer(cfg *config.Config, router chi.Router) *http.Server {
	return &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}
}
