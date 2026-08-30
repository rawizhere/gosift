package parser

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/models"
	"github.com/rawizhere/gosift/internal/repo"
)

type Sender interface {
	SendCards(ctx context.Context, chatID int64, offers []models.Offer) error
	SendAlert(ctx context.Context, chatID int64, store string, err error) error
}

type Engine struct {
	store     *repo.Store
	registry  *Registry
	sender    Sender
	cfg       *config.Config
	log       *slog.Logger
	lastAlert map[string]time.Time
}

func NewEngine(store *repo.Store, registry *Registry, sender Sender, cfg *config.Config, log *slog.Logger) *Engine {
	return &Engine{
		store:     store,
		registry:  registry,
		sender:    sender,
		cfg:       cfg,
		log:       log,
		lastAlert: map[string]time.Time{},
	}
}

func (e *Engine) RunOnce(ctx context.Context) error {
	rules, err := e.store.ListEnabledRules(ctx)
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}
	byChat := map[int64][]models.Offer{}
	for _, rule := range rules {
		e.jitterSleep(ctx)
		offers, err := e.parseRule(ctx, rule)
		if err != nil {
			e.alert(rule, err)
			continue
		}
		byChat[rule.ChatID] = append(byChat[rule.ChatID], offers...)
	}
	for chatID, offers := range byChat {
		offers = dedupe(offers)
		if len(offers) == 0 {
			continue
		}
		if err := e.sender.SendCards(ctx, chatID, offers); err != nil {
			e.log.Error("send cards", "chat", chatID, "error", err)
		}
	}
	return nil
}

func (e *Engine) parseRule(ctx context.Context, rule models.Rule) (offers []models.Offer, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in parse rule: %v", r)
		}
	}()
	p, err := e.registry.Get(rule.Store)
	if err != nil {
		return nil, err
	}
	raw, err := p.Search(ctx, rule, SearchOptions{Limit: e.cfg.ParseLimit})
	if err != nil {
		return nil, err
	}
	out := make([]models.Offer, 0, len(raw))
	for _, o := range raw {
		if e.matches(rule, o) {
			out = append(out, o)
		}
	}
	return out, nil
}

func cityMatch(ruleCity, offerCity string) bool {
	a := strings.ToLower(strings.TrimSpace(ruleCity))
	b := strings.ToLower(strings.TrimSpace(offerCity))
	return a == b || strings.Contains(b, a) || strings.Contains(a, b)
}

func (e *Engine) matches(rule models.Rule, offer models.Offer) bool {
	if !priceInRange(rule.MinPrice, rule.MaxPrice, offer.Price) {
		return false
	}
	if rule.City != "" && !cityMatch(rule.City, offer.City) {
		return false
	}
	query := strings.ToLower(strings.TrimSpace(rule.Query))
	title := strings.ToLower(offer.Title)
	return query == "" || strings.Contains(title, query)
}

func (e *Engine) jitterSleep(ctx context.Context) {
	if e.cfg.ParseJitter <= 0 {
		return
	}
	delay := time.Duration(rand.Int64N(int64(e.cfg.ParseJitter)))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (e *Engine) alert(rule models.Rule, err error) {
	e.log.Error("parse rule failed", "rule", rule.ID, "store", rule.Store, "error", err)
	key := fmt.Sprintf("%d:%s", rule.ChatID, rule.Store)
	last := e.lastAlert[key]
	if time.Since(last) < e.cfg.NotifyAlertInterval {
		return
	}
	e.lastAlert[key] = time.Now()
	if err := e.sender.SendAlert(context.Background(), rule.ChatID, rule.Store, err); err != nil {
		e.log.Error("send alert", "chat", rule.ChatID, "error", err)
	}
}

func priceInRange(minStr, maxStr, priceStr string) bool {
	if priceStr == "" {
		return false
	}
	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		return false
	}
	if minStr != "" {
		min, err := decimal.NewFromString(minStr)
		if err == nil && price.LessThan(min) {
			return false
		}
	}
	if maxStr != "" {
		max, err := decimal.NewFromString(maxStr)
		if err == nil && price.GreaterThan(max) {
			return false
		}
	}
	return true
}

func dedupe(offers []models.Offer) []models.Offer {
	seen := map[string]struct{}{}
	out := make([]models.Offer, 0, len(offers))
	for _, o := range offers {
		key := o.Store + "|" + o.URL + "|" + o.Title
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, o)
	}
	return out
}
