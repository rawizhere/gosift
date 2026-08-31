package parser

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/models"
	"github.com/rawizhere/gosift/internal/randutil"
	"github.com/rawizhere/gosift/internal/repo"
)

const consecutiveFailures = 3

type Sender interface {
	SendCards(ctx context.Context, chatID int64, offers []models.Offer) error
	SendAlert(ctx context.Context, chatID int64, store string, err error) error
}

type Engine struct {
	repo      *repo.Store
	registry  *Registry
	sender    Sender
	cfg       *config.Config
	log       *slog.Logger
	lastAlert map[string]time.Time
	fails     map[string]int
}

func NewEngine(store *repo.Store, registry *Registry, sender Sender, cfg *config.Config, log *slog.Logger) *Engine {
	return &Engine{
		repo:      store,
		registry:  registry,
		sender:    sender,
		cfg:       cfg,
		log:       log,
		lastAlert: map[string]time.Time{},
		fails:     map[string]int{},
	}
}

func (e *Engine) RunOnce(ctx context.Context) error {
	rules, err := e.repo.ListEnabledRules(ctx)
	if err != nil {
		return fmt.Errorf("list rules: %w", err)
	}
	seen := map[string]bool{}
	for _, rule := range rules {
		e.jitterSleep(ctx)
		offers, err := e.parseRule(ctx, rule)
		if err != nil {
			e.alert(rule, err)
			continue
		}
		e.fails[alertKey(rule)] = 0
		unique := make([]models.Offer, 0, len(offers))
		for _, o := range offers {
			key := fmt.Sprintf("%d|%s", rule.ChatID, offerKey(o))
			if seen[key] {
				continue
			}
			seen[key] = true
			unique = append(unique, o)
		}
		if len(unique) == 0 {
			continue
		}
		// Persistent dedup: send only new listings and price drops.
		toSend := make([]models.Offer, 0, len(unique))
		for _, o := range unique {
			should, err := e.repo.ShouldNotifyOffer(ctx, rule.ChatID, offerKey(o), o.Price.String())
			if err != nil {
				e.log.Error("dedup check failed", "rule", rule.ID, "chat", rule.ChatID, "offer", offerKey(o), "error", err)
				continue
			}
			if should {
				toSend = append(toSend, o)
			}
		}
		if len(toSend) == 0 {
			continue
		}
		sortOffers(toSend)
		if err := e.sender.SendCards(ctx, rule.ChatID, toSend); err != nil {
			e.log.Error("send cards", "chat", rule.ChatID, "error", err)
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
	raw, err := p.Search(ctx, rule, SearchOptions{Query: positiveQuery(rule.Query), Limit: e.cfg.ParseLimit})
	if err != nil {
		return nil, err
	}
	out := make([]models.Offer, 0, len(raw))
	for _, o := range raw {
		if e.matches(rule, o) {
			out = append(out, o)
		}
	}
	sortOffers(out)
	return out, nil
}

func positiveQuery(q string) string {
	pos, _ := splitQuery(q)
	return pos
}

func splitQuery(q string) (string, []string) {
	var pos []string
	var neg []string
	for _, t := range strings.Fields(q) {
		if strings.HasPrefix(t, "-") {
			if t = strings.TrimPrefix(t, "-"); t != "" {
				neg = append(neg, strings.ToLower(t))
			}
			continue
		}
		pos = append(pos, t)
	}
	return strings.TrimSpace(strings.Join(pos, " ")), neg
}

func (e *Engine) matches(rule models.Rule, offer models.Offer) bool {
	if !priceInRange(rule.MinPrice, rule.MaxPrice, offer.Price) {
		return false
	}
	if rule.City != "" && !cityMatch(rule.City, offer.City) {
		return false
	}
	pos, neg := splitQuery(rule.Query)
	// Match words in both the title and the description.
	text := strings.ToLower(offer.Title + " " + offer.Description)
	if pos != "" && !strings.Contains(text, pos) {
		return false
	}
	for _, n := range neg {
		if strings.Contains(text, n) {
			return false
		}
	}
	return true
}

func cityMatch(ruleCity, offerCity string) bool {
	a := strings.ToLower(strings.TrimSpace(ruleCity))
	b := strings.ToLower(strings.TrimSpace(offerCity))
	return a == b || strings.Contains(b, a) || strings.Contains(a, b)
}

func priceInRange(min, max *decimal.Decimal, price decimal.Decimal) bool {
	if min != nil && price.LessThan(*min) {
		return false
	}
	if max != nil && price.GreaterThan(*max) {
		return false
	}
	return true
}

func sortOffers(offers []models.Offer) {
	sort.SliceStable(offers, func(i, j int) bool {
		return offers[i].Price.LessThan(offers[j].Price)
	})
}

func offerKey(o models.Offer) string {
	if o.Key != "" {
		return o.Store + "|" + o.Key
	}
	return o.Store + "|" + o.URL + "|" + o.Title
}

func alertKey(rule models.Rule) string {
	return fmt.Sprintf("%d:%s", rule.ChatID, rule.Store)
}

func (e *Engine) jitterSleep(ctx context.Context) {
	if e.cfg.ParseJitter <= 0 {
		return
	}
	timer := time.NewTimer(randutil.Duration(e.cfg.ParseJitter))
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (e *Engine) alert(rule models.Rule, err error) {
	e.log.Error("parse rule failed", "rule", rule.ID, "store", rule.Store, "error", err)
	key := alertKey(rule)
	e.fails[key]++
	if e.fails[key] < consecutiveFailures {
		return
	}
	if time.Since(e.lastAlert[key]) < e.cfg.NotifyAlertInterval {
		return
	}
	e.lastAlert[key] = time.Now()
	e.fails[key] = 0
	if err := e.sender.SendAlert(context.Background(), rule.ChatID, rule.Store, err); err != nil {
		e.log.Error("send alert", "chat", rule.ChatID, "error", err)
	}
}
