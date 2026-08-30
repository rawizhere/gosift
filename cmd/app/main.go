package main

import (
	"fmt"
	"os"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/db"
	"github.com/rawizhere/gosift/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log := logger.New(cfg)
	sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Error("open db", "error", err)
		os.Exit(1)
	}
	defer func() { _ = sqlDB.Close() }()
	log.Info("started", "db", cfg.DBPath)
}
