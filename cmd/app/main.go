package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/db"
	"github.com/rawizhere/gosift/internal/httpclient"
	"github.com/rawizhere/gosift/internal/logger"
	"github.com/rawizhere/gosift/internal/repo"
	"github.com/rawizhere/gosift/internal/scheduler"
	"github.com/rawizhere/gosift/internal/telegram"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg)
	sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	store := repo.NewStore(sqlDB)
	hc, err := httpclient.NewRetryable(cfg)
	if err != nil {
		return err
	}
	bot, err := telegram.New(cfg, store, log, hc)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return scheduler.Run(ctx, cfg.ParseInterval, cfg.ParseJitter, func() {
			log.Info("parse cycle")
		})
	})
	g.Go(func() error {
		return bot.Run(ctx)
	})
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
