package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/httpclient"
	"github.com/rawizhere/gosift/internal/repo"
)

type Bot struct {
	bot     *telego.Bot
	store   *repo.Store
	cfg     *config.Config
	log     *slog.Logger
	allowed map[int64]bool
	stores  []storeOption
}

func New(cfg *config.Config, store *repo.Store, log *slog.Logger, hc *httpclient.Client, storeNames []string) (*Bot, error) {
	bot, err := telego.NewBot(cfg.TelegramBotToken,
		telego.WithHTTPClient(hc.StandardClient()),
		telego.WithLogger(slogAdapter{log}),
	)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}
	opts := make([]storeOption, 0, len(storeNames))
	for _, name := range storeNames {
		opts = append(opts, storeOption{Key: name, Label: name})
	}
	return &Bot{
		bot:     bot,
		store:   store,
		cfg:     cfg,
		log:     log,
		allowed: parseAllowed(cfg.TelegramAllowedUsers),
		stores:  opts,
	}, nil
}

func (b *Bot) Run(ctx context.Context) error {
	updates, err := b.bot.UpdatesViaLongPolling(ctx, nil)
	if err != nil {
		return fmt.Errorf("long polling: %w", err)
	}
	for update := range updates {
		b.handle(ctx, update)
	}
	return nil
}

func (b *Bot) handle(ctx context.Context, update telego.Update) {
	if update.Message != nil && update.Message.Text != "" {
		b.handleMessage(ctx, update.Message)
		return
	}
	if update.CallbackQuery != nil {
		b.handleCallback(ctx, update.CallbackQuery)
	}
}

func (b *Bot) allowedUser(userID int64) bool {
	if len(b.allowed) == 0 {
		return true
	}
	return b.allowed[userID]
}

func parseAllowed(raw string) map[int64]bool {
	m := map[int64]bool{}
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil {
			m[id] = true
		}
	}
	return m
}
