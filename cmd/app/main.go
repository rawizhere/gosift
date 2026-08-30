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
	"github.com/rawizhere/gosift/internal/parser"
	"github.com/rawizhere/gosift/internal/parsers/komissionki"
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
	registry := parser.NewRegistry()
	if cfg.StoreEnabled {
		if err := registry.Register(komissionki.New(hc, cfg.StoreBaseURL, cfg.StoreAPIURL)); err != nil {
			return err
		}
	}
	bot, err := telegram.New(cfg, store, log, hc, registry.Names())
	if err != nil {
		return err
	}
	engine := parser.NewEngine(store, registry, bot, cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return scheduler.Run(ctx, cfg.ParseInterval, cfg.ParseJitter, func() {
			if err := engine.RunOnce(ctx); err != nil {
				log.Error("parse cycle failed", "error", err)
			}
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
