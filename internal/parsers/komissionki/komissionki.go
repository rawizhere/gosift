package komissionki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rawizhere/gosift/internal/httpclient"
	"github.com/rawizhere/gosift/internal/models"
	"github.com/rawizhere/gosift/internal/parser"
)

const (
	pageSize = 36
	// maxPages bounds one scan; the busiest feed moves ~4 pages per 15 minutes.
	maxPages = 20
)

var errUnauthorized = errors.New("api status 401")

type apiResponse struct {
	Data apiData `json:"data"`
}

type apiData struct {
	Items []apiItem `json:"data"`
	SA    string    `json:"sa"`
}

type apiItem struct {
	ID              int64  `json:"id"`
	Code            string `json:"code"`
	BarcodeShopUniq string `json:"barcodeShopUniq"`
	Name            string `json:"name"`
	// Title is what the site shows; Name is the original and can be stale.
	Title          string       `json:"title"`
	UpdatedAt      time.Time    `json:"updatedAt"`
	Description    string       `json:"description"`
	Price          json.Number  `json:"price"`
	OldPrice       *json.Number `json:"oldPrice"`
	City           []apiCity    `json:"city"`
	Category       apiCategory  `json:"category"`
	ParentCategory apiCategory  `json:"parentCategory"`
	// PhotosResized maps photo IDs to gallery positions.
	PhotosResized map[string]int `json:"photosResized"`
}

type apiCity struct {
	Name string `json:"name"`
}

type apiCategory struct {
	Code  string `json:"code"`
	Title string `json:"title"`
	Name  string `json:"name"`
}

type Parser struct {
	client  *httpclient.Client
	baseURL string
	apiURL  string
	cdnURL  string
}

func New(client *httpclient.Client, baseURL, apiURL, cdnURL string) *Parser {
	return &Parser{client: client, baseURL: baseURL, apiURL: apiURL, cdnURL: cdnURL}
}

func (p *Parser) Name() string {
	return "komissionki"
}

// Search scans the store feed newest-updated first, so new listings and price changes both surface,
// and stops at opts.Since. The query is matched by the engine: with a search term the API ranks
// by relevance and ignores the sort order.
func (p *Parser) Search(ctx context.Context, rule models.Rule, opts parser.SearchOptions) ([]models.Offer, error) {
	var offers []models.Offer
	sa := ""
	for page := 1; page <= maxPages; page++ {
		items, nextSA, err := p.fetch(ctx, rule, page, sa)
		if page > 1 && (errors.Is(err, errUnauthorized) || err == nil && len(items) == 0) {
			// The session token expired: mint a new one on page one and retry once.
			if _, sa, err = p.fetch(ctx, rule, 1, ""); err == nil {
				items, nextSA, err = p.fetch(ctx, rule, page, sa)
			}
		}
		if err != nil {
			return nil, err
		}
		if nextSA != "" {
			sa = nextSA
		}
		for _, item := range items {
			if item.UpdatedAt.Before(opts.Since) {
				return offers, nil
			}
			offers = append(offers, toOffer(p.baseURL, p.cdnURL, item))
		}
		if len(items) < pageSize {
			break
		}
	}
	return offers, nil
}

// fetch requests one feed page with the rule's category and price filters.
func (p *Parser) fetch(ctx context.Context, rule models.Rule, page int, sa string) ([]apiItem, string, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(pageSize))
	params.Set("page", strconv.Itoa(page))
	params.Set("sort", "desc")
	params.Set("fieldSort", "updatedAt")
	if category, isParent := ruleCategoryParams(rule.Category); category != "" {
		params.Set("category", category)
		if isParent {
			params.Set("isParentCategory", "1")
		}
	}
	if rule.MinPrice != nil {
		params.Set("priceFrom", rule.MinPrice.String())
	}
	if rule.MaxPrice != nil {
		params.Set("priceTo", rule.MaxPrice.String())
	}
	if sa != "" {
		params.Set("sa", sa)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL+"/api/product-filter/filter?"+params.Encode(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := p.client.Do(ctx, req, p.Name())
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, "", errUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("api status %d", resp.StatusCode)
	}
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", fmt.Errorf("decode: %w", err)
	}
	return out.Data.Items, out.Data.SA, nil
}

func toOffer(baseURL, cdnURL string, item apiItem) models.Offer {
	city := ""
	if len(item.City) > 0 {
		city = item.City[0].Name
	}
	price := decimal.Zero
	if p, err := decimal.NewFromString(item.Price.String()); err == nil {
		price = p
	}
	var oldPrice *decimal.Decimal
	if item.OldPrice != nil {
		if d, err := decimal.NewFromString(item.OldPrice.String()); err == nil {
			oldPrice = &d
		}
	}
	u := fmt.Sprintf("%s/catalog/%s/%s/%s/", baseURL, item.ParentCategory.Code, item.Category.Code, item.Code)
	title := item.Title
	if title == "" {
		title = item.Name
	}
	catTitle := item.Category.Title
	if catTitle == "" {
		catTitle = item.Category.Name
	}
	return models.Offer{
		Key:         item.BarcodeShopUniq,
		Store:       "komissionki",
		Category:    catTitle,
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(item.Description),
		URL:         u,
		Price:       price,
		OldPrice:    oldPrice,
		City:        strings.TrimSpace(city),
		Images:      imageURLs(cdnURL, item.PhotosResized),
	}
}

const maxAlbumPhotos = 10

// imageURLs builds CDN URLs for product photos by gallery position.
func imageURLs(cdnURL string, photos map[string]int) []string {
	if cdnURL == "" || len(photos) == 0 {
		return nil
	}
	ids := make([]string, 0, len(photos))
	for id := range photos {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if photos[ids[i]] != photos[ids[j]] {
			return photos[ids[i]] < photos[ids[j]]
		}
		return ids[i] < ids[j]
	})
	cdn := strings.TrimRight(cdnURL, "/")
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if len(out) >= maxAlbumPhotos {
			break
		}
		out = append(out, fmt.Sprintf("%s/%s/%s_XL.webp", cdn, id, id))
	}
	return out
}

// ruleCategoryParams maps a rule category path to API filter params.
func ruleCategoryParams(path string) (category string, isParent bool) {
	if path == "" {
		return "", false
	}
	parts := strings.SplitN(path, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	kind, id := parts[0], parts[1]
	if id == "" {
		return "", false
	}
	switch kind {
	case "parent":
		return id, true
	case "child":
		return id, false
	}
	return "", false
}
