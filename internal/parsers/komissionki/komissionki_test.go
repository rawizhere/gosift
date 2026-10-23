package komissionki

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestSearchScansFeedUntilSince(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	// two full pages and a third, updated one minute apart, newest first
	feed := make([]map[string]any, 2*pageSize+5)
	for i := range feed {
		feed[i] = map[string]any{
			"barcodeShopUniq": strconv.Itoa(i),
			"name":            "old name",
			"title":           "Title " + strconv.Itoa(i),
			"price":           1000,
			"updatedAt":       now.Add(-time.Duration(i) * time.Minute),
		}
	}
	var queries []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		queries = append(queries, q)
		page, _ := strconv.Atoi(q.Get("page"))
		if page > 1 && q.Get("sa") != "token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		lo := min((page-1)*pageSize, len(feed))
		hi := min(lo+pageSize, len(feed))
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"data": feed[lo:hi], "sa": "token"}})
	}))
	defer srv.Close()

	cfg := &config.Config{ParseRetries: 0, ParseRetryBackoff: time.Millisecond, ParseInterval: time.Hour, ParseTimeout: 5 * time.Second, StoreRPS: 1000}
	hc, err := httpclient.New(cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	p := New(hc, "https://komissionki.ru", srv.URL, "")
	max := decimal.NewFromInt(5000)
	// cut-off falls on page two
	offers, err := p.Search(context.Background(), models.Rule{Query: "ignored by api", MaxPrice: &max},
		parser.SearchOptions{Since: now.Add(-time.Duration(pageSize+3)*time.Minute + time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != pageSize+3 {
		t.Fatalf("offers = %d, want %d", len(offers), pageSize+3)
	}
	if offers[0].Title != "Title 0" {
		t.Errorf("title = %q, want the site title", offers[0].Title)
	}
	if len(queries) != 2 {
		t.Fatalf("requests = %d, want 2", len(queries))
	}
	q := queries[0]
	if q.Has("search") || q.Get("fieldSort") != "updatedAt" || q.Get("priceTo") != "5000" {
		t.Errorf("query = %v", q)
	}
}

func TestToOfferTitleFallsBackToName(t *testing.T) {
	o := toOffer("https://komissionki.ru", "", apiItem{Name: " Pixel "})
	if o.Title != "Pixel" {
		t.Errorf("title = %q", o.Title)
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
		StoreBaseURL:  "https://komissionki.ru",
		StoreAPIURL:   "https://saf.komissionki.ru",
		StoreCDNURL:   "https://c.komissionki.ru",
		ParseTimeout:  20e9,
		ParseInterval: time.Hour,
		ParseRetries:  2,
		StoreRPS:      0.5,
	}
	hc, err := httpclient.New(cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	p := New(hc, cfg.StoreBaseURL, cfg.StoreAPIURL, cfg.StoreCDNURL)

	min := decimal.NewFromInt(50000)
	offers, err := p.Search(context.Background(), models.Rule{
		Query:    "macbook pro",
		MinPrice: &min,
	}, parser.SearchOptions{Since: time.Now().Add(-time.Hour)})
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
