package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/httpclient"
)

func main() {
	cfg := &config.Config{
		TelegramBotToken: "x",
		ParseTimeout:     10 * time.Second,
		ParseRetries:     1,
		ParseRetryBackoff: 100 * time.Millisecond,
		UserAgent:        "test-agent",
		StoreRPS:         2,
	}
	c, err := httpclient.NewRetryable(cfg)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	req, err := c.Request(context.Background(), "GET", "https://saf.komissionki.ru/api/product-filter/filter?search=macbook&limit=1&page=1")
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	start := time.Now()
	resp, err := c.Do(req)
	if err != nil {
		fmt.Println("ERR", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	fmt.Println("status:", resp.StatusCode, "in", time.Since(start).Round(time.Millisecond))
}
