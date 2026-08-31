package parser

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/db"
	"github.com/rawizhere/gosift/internal/models"
	"github.com/rawizhere/gosift/internal/repo"
)

type fakeParser struct {
	name   string
	offers []models.Offer
}

func (f *fakeParser) Name() string { return f.name }

func (f *fakeParser) Search(_ context.Context, _ models.Rule, _ SearchOptions) ([]models.Offer, error) {
	return f.offers, nil
}

type fakeSender struct {
	sent       []models.Offer
	alertCount int
}

func (s *fakeSender) SendCards(_ context.Context, _ int64, offers []models.Offer) error {
	s.sent = append(s.sent, offers...)
	return nil
}

func (s *fakeSender) SendAlert(_ context.Context, _ int64, _ string, _ error) error {
	s.alertCount++
	return nil
}

func newTestEngine(t *testing.T, p Parser) (*Engine, *fakeSender, *repo.Store) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := repo.NewStore(sqlDB)
	reg := NewRegistry()
	if err := reg.Register(p); err != nil {
		t.Fatal(err)
	}
	sender := &fakeSender{}
	cfg := &config.Config{ParseLimit: 20, ParseJitter: 0, NotifyAlertInterval: 1e9}
	log := slog.New(slog.NewTextHandler(discard{}, nil))
	return NewEngine(store, reg, sender, cfg, log), sender, store
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func addRule(t *testing.T, store *repo.Store) models.Rule {
	t.Helper()
	if err := store.CreateUser(context.Background(), models.User{UserID: 1, ChatID: 1}); err != nil {
		t.Fatal(err)
	}
	r := models.Rule{UserID: 1, ChatID: 1, Store: "komissionki", Query: "macbook pro", Enabled: true}
	if err := store.CreateRule(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	rules, err := store.ListEnabledRules(context.Background())
	if err != nil || len(rules) != 1 {
		t.Fatalf("list rules: %v (%d)", err, len(rules))
	}
	return rules[0]
}

func offerWithPrice(key string, price int64) models.Offer {
	return models.Offer{Key: key, Store: "komissionki", Title: "MacBook Pro", Price: decimal.NewFromInt(price), URL: "https://x/" + key}
}

func TestEngineSendsOnlyNewAndPriceDrops(t *testing.T) {
	fp := &fakeParser{name: "komissionki"}
	engine, sender, store := newTestEngine(t, fp)
	ctx := context.Background()
	addRule(t, store)

	// cycle 1: two new offers -> both sent
	fp.offers = []models.Offer{offerWithPrice("b1", 1000), offerWithPrice("b2", 2000)}
	if err := engine.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 2 {
		t.Fatalf("cycle 1: sent %d, want 2", len(sender.sent))
	}

	// cycle 2: same listings -> nothing new
	fp.offers = []models.Offer{offerWithPrice("b1", 1000), offerWithPrice("b2", 2000)}
	if err := engine.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 2 {
		t.Fatalf("cycle 2: sent %d, want still 2 (no resend)", len(sender.sent))
	}

	// cycle 3: b2 dropped, b3 is new, b1 unchanged -> b2 and b3
	fp.offers = []models.Offer{
		offerWithPrice("b1", 1000), // unchanged -> no resend
		offerWithPrice("b2", 1500), // dropped 2000 -> send
		offerWithPrice("b3", 500),  // new -> send
	}
	if err := engine.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, o := range sender.sent {
		got[o.Key] = true
	}
	if len(sender.sent) != 4 || !got["b2"] || !got["b3"] {
		t.Fatalf("cycle 3: sent keys = %v", mapKeys(sender.sent))
	}
}

// mapKeys returns the Offer keys of all captured sends.
func mapKeys(offers []models.Offer) []string {
	keys := make([]string, 0, len(offers))
	for _, o := range offers {
		keys = append(keys, o.Key)
	}
	return keys
}

func TestMatchesSearchesDescription(t *testing.T) {
	e := &Engine{}
	rule := models.Rule{Query: "iphone -трещина"}

	// positive word only in description
	o := models.Offer{Title: "Apple iPhone Air 12/256", Description: "в комплекте коробка, чехол"}
	if !e.matches(rule, o) {
		t.Errorf("expected match via description")
	}
	// negative word only in description must reject
	o2 := models.Offer{Title: "Apple iPhone Air 12/256", Description: "на корпусе трещина"}
	if e.matches(rule, o2) {
		t.Errorf("expected rejection via description")
	}
	// no match anywhere
	o3 := models.Offer{Title: "Ноутбук Lenovo", Description: "б/у"}
	if e.matches(rule, o3) {
		t.Errorf("expected no match")
	}
}

func TestMatchesTitleAndPrice(t *testing.T) {
	e := &Engine{}
	min := decimal.NewFromInt(1000)
	rule := models.Rule{Query: "macbook pro", MinPrice: &min}
	o := models.Offer{Title: "MacBook Pro 2019", Price: decimal.NewFromInt(1500)}
	if !e.matches(rule, o) {
		t.Errorf("expected match")
	}
	o2 := models.Offer{Title: "MacBook Pro 2019", Price: decimal.NewFromInt(900)}
	if e.matches(rule, o2) {
		t.Errorf("expected price rejection")
	}
}

func TestPositiveQuery(t *testing.T) {
	got := positiveQuery("macbook pro -neo -следы")
	if got != "macbook pro" {
		t.Errorf("positiveQuery = %q", got)
	}
}

func TestSplitQuery(t *testing.T) {
	pos, neg := splitQuery("  MacBook Pro  -neo  -трещина ")
	if !strings.EqualFold(pos, "MacBook Pro") {
		t.Errorf("pos = %q", pos)
	}
	if len(neg) != 2 || neg[0] != "neo" || neg[1] != "трещина" {
		t.Errorf("neg = %v", neg)
	}
}
