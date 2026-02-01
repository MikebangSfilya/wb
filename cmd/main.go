package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/MikebangSfilya/wb/internal/config"
	redis2 "github.com/MikebangSfilya/wb/internal/repository/redis"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	_ = cfg
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// TODO: init storage

	r, err := redis2.New(ctx, cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		panic(err)
	}

	//TODO: init kafka

	// TODO: init route
	//TODO: start srv
	defer r.Close()
}
