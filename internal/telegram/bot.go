package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/httpclient"
	"github.com/rawizhere/gosift/internal/repo"
)

type Bot struct {
	bot      *telego.Bot
	repo     *repo.Store
	cfg      *config.Config
	log      *slog.Logger
	hc       *httpclient.Client
	cdnHosts []string
	allowed  map[int64]bool
	stores   []string
}

func New(cfg *config.Config, store *repo.Store, log *slog.Logger, hc *httpclient.Client, storeNames []string) (*Bot, error) {
	bot, err := telego.NewBot(cfg.TelegramBotToken,
		telego.WithHTTPClient(hc.StandardClient()),
		telego.WithLogger(slogAdapter{log}),
	)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}
	stores := make([]string, len(storeNames))
	copy(stores, storeNames)
	return &Bot{
		bot:      bot,
		repo:     store,
		cfg:      cfg,
		log:      log,
		hc:       hc,
		cdnHosts: cdnHosts(cfg),
		allowed:  parseAllowed(cfg.TelegramAllowedUsers),
		stores:   stores,
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

// cdnHosts returns the ordered list of image CDN hosts, primary first,
// without duplicates.
func cdnHosts(cfg *config.Config) []string {
	var out []string
	seen := map[string]bool{}
	add := func(raw string) {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Host == "" {
			return
		}
		if !seen[u.Host] {
			seen[u.Host] = true
			out = append(out, u.Host)
		}
	}
	add(cfg.StoreCDNURL)
	for _, raw := range strings.Split(cfg.StoreCDNFallbacks, ",") {
		add(raw)
	}
	return out
}
