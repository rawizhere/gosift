package parser

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/rawizhere/gosift/internal/models"
)

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
