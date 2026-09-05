package komissionki

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/rawizhere/gosift/internal/config"
	"github.com/rawizhere/gosift/internal/httpclient"
	"github.com/rawizhere/gosift/internal/models"
	"github.com/rawizhere/gosift/internal/parser"
)

func TestImageURLsSortedByPosition(t *testing.T) {
	photos := map[string]int{
		"c": 2,
		"a": 0,
		"b": 1,
	}
	got := imageURLs("https://c.example.com/", photos)
	want := []string{
		"https://c.example.com/a/a_XL.webp",
		"https://c.example.com/b/b_XL.webp",
		"https://c.example.com/c/c_XL.webp",
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("url[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestImageURLsCap(t *testing.T) {
	photos := map[string]int{}
	for i := 0; i < 15; i++ {
		photos[string(rune('a'+i))] = i
	}
	got := imageURLs("https://c.example.com", photos)
	if len(got) != maxAlbumPhotos {
		t.Fatalf("len = %d, want %d", len(got), maxAlbumPhotos)
	}
}

func TestImageURLsEmpty(t *testing.T) {
	if got := imageURLs("", map[string]int{"a": 0}); got != nil {
		t.Fatalf("empty cdn: got %v, want nil", got)
	}
	if got := imageURLs("https://c.example.com", nil); got != nil {
		t.Fatalf("nil photos: got %v, want nil", got)
	}
}

func TestOffersFromItemsLimit(t *testing.T) {
	items := make([]apiItem, 5)
	for i := range items {
		items[i] = apiItem{Code: "x", Category: apiCategory{Code: "c"}, ParentCategory: apiCategory{Code: "p"}}
	}
	got := offersFromItems("https://komissionki.ru", "https://c.example.com", items, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestToOfferDescriptionAndImages(t *testing.T) {
	item := apiItem{
		Code:            "item-1",
		BarcodeShopUniq: "key-1",
		Name:            "Ноутбук  MacBook Pro",
		Description:     "  в комплекте коробка, чехол  ",
		Price:           "109990",
		City:            []apiCity{{Name: "Москва"}},
		Filial:          []apiFilial{{Balance: 1}},
		Category:        apiCategory{Code: "noutbuki"},
		PhotosResized:   map[string]int{"p1": 0},
	}
	o := toOffer("https://komissionki.ru", "https://c.example.com", item)
	if o.Description != "в комплекте коробка, чехол" {
		t.Errorf("description = %q", o.Description)
	}
	if len(o.Images) != 1 || !strings.HasSuffix(o.Images[0], "/p1/p1_XL.webp") {
		t.Errorf("images = %v", o.Images)
	}
}

// TestSearchLive hits the real komissionki API; set GOSIFT_LIVE=1 to run.
func TestSearchLive(t *testing.T) {
	if os.Getenv("GOSIFT_LIVE") == "" {
		t.Skip("set GOSIFT_LIVE=1 to run live API tests")
	}
	cfg := &config.Config{
		StoreBaseURL: "https://komissionki.ru",
		StoreAPIURL:  "https://saf.komissionki.ru",
		StoreCDNURL:  "https://c.komissionki.ru",
		ParseTimeout: 20e9,
		ParseRetries: 2,
		StoreRPS:     0.5,
	}
	hc, err := httpclient.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	p := New(hc, cfg.StoreBaseURL, cfg.StoreAPIURL, cfg.StoreCDNURL)

	min := decimal.NewFromInt(50000)
	offers, err := p.Search(context.Background(), models.Rule{
		Query:    "macbook pro",
		MinPrice: &min,
	}, parser.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("search with price filter: %v", err)
	}
	if len(offers) == 0 {
		t.Fatal("no offers returned")
	}
	imageHosts := []string{"c.komissionki.ru", "cdny.komissionki.ru"}
	for _, o := range offers {
		if o.Price.LessThan(min) {
			t.Errorf("offer %s price %s below filter", o.Key, o.Price)
		}
		if len(o.Images) == 0 {
			t.Errorf("offer %s has no images", o.Key)
			continue
		}
		// images may live on several CDN hosts; at least one must load
		var got []byte
		for _, host := range imageHosts {
			u := strings.Replace(o.Images[0], "c.komissionki.ru", host, 1)
			data, err := hc.GetBytes(context.Background(), u)
			if err == nil {
				got = data
				break
			}
		}
		if len(got) == 0 {
			t.Errorf("first image of %s could not be downloaded from any host", o.Key)
		}
	}
}

func TestRuleCategoryParams(t *testing.T) {
	if cat, parent := ruleCategoryParams(""); cat != "" || parent {
		t.Fatalf("empty path: got %q %v", cat, parent)
	}
	if cat, parent := ruleCategoryParams("parent:37"); cat != "37" || !parent {
		t.Fatalf("parent path: got %q %v", cat, parent)
	}
	if cat, parent := ruleCategoryParams("child:45"); cat != "45" || parent {
		t.Fatalf("child path: got %q %v", cat, parent)
	}
	if cat, parent := ruleCategoryParams("garbage"); cat != "" || parent {
		t.Fatalf("garbage path: got %q %v", cat, parent)
	}
}
