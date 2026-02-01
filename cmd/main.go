package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MikebangSfilya/wb/internal/config"
	sl2 "github.com/MikebangSfilya/wb/internal/lib/log"
	redis2 "github.com/MikebangSfilya/wb/internal/repository/redis"
	"github.com/MikebangSfilya/wb/internal/storage/postgre"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/sync/errgroup"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	sl := sl2.SetupLogger(cfg.Env)
	slog.SetDefault(sl)
	sl.Info("config loaded, start application")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgre.New(ctx, cfg)
	if err != nil {
		sl.Error("Database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	r, err := redis2.New(ctx, cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		sl.Error("Redis connection failed", "error", err)
		os.Exit(1)
	}
	defer r.Close()

	//TODO: init kafka

	// TODO: init route
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	srv := newServer(cfg, router)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
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
