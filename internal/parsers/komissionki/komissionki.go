package komissionki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/rawizhere/gosift/internal/httpclient"
	"github.com/rawizhere/gosift/internal/models"
	"github.com/rawizhere/gosift/internal/parser"
)

const pageSize = 36

var errUnauthorized = errors.New("api status 401")

type apiResponse struct {
	Data apiData `json:"data"`
}

type apiData struct {
	Filters []json.RawMessage `json:"filters"`
	Items   []apiItem         `json:"data"`
	SA      string            `json:"sa"`
	Total   int               `json:"total"`
}

type apiItem struct {
	ID              int64        `json:"id"`
	Code            string       `json:"code"`
	BarcodeShopUniq string       `json:"barcodeShopUniq"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	Price           json.Number  `json:"price"`
	OldPrice        *json.Number `json:"oldPrice"`
	City            []apiCity    `json:"city"`
	Filial          []apiFilial  `json:"filial"`
	Category        apiCategory  `json:"category"`
	ParentCategory  apiCategory  `json:"parentCategory"`
	// PhotosResized maps photo IDs to gallery positions.
	PhotosResized map[string]int `json:"photosResized"`
}

type apiCity struct {
	Name string `json:"name"`
}

type apiFilial struct {
	Balance int `json:"balance"`
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

func (p *Parser) Search(ctx context.Context, rule models.Rule, opts parser.SearchOptions) ([]models.Offer, error) {
	query := opts.Query
	if query == "" {
		query = rule.Query
	}
	endpoint := p.apiURL + "/api/product-filter/filter"

	// The first page mints the session token; price filters may return none.
	first, sa, err := p.handshake(ctx, endpoint, query, rule)
	if err != nil {
		return nil, fmt.Errorf("handshake: %w", err)
	}
	offers := offersFromItems(p.baseURL, p.cdnURL, first, opts.Limit)

	// Pagination needs a token the API only mints for unfiltered searches.
	priceFiltered := rule.MinPrice != nil || rule.MaxPrice != nil
	for page := 2; !priceFiltered && len(offers) < opts.Limit; page++ {
		raw, nextSA, err := p.fetchPage(ctx, endpoint, query, page, sa, rule)
		if err != nil {
			if !errors.Is(err, errUnauthorized) {
				return nil, err
			}
			// Session expired: re-handshake and retry the page once.
			_, sa, err = p.handshake(ctx, endpoint, query, rule)
			if err != nil {
				return nil, err
			}
			raw, nextSA, err = p.fetchPage(ctx, endpoint, query, page, sa, rule)
			if err != nil {
				return nil, err
			}
		}
		sa = nextSA
		if len(raw) == 0 {
			_, reSA, err := p.handshake(ctx, endpoint, query, rule)
			if err != nil {
				break
			}
			sa = reSA
			raw, nextSA, err = p.fetchPage(ctx, endpoint, query, page, sa, rule)
			if err != nil {
				return nil, err
			}
			sa = nextSA
			if len(raw) == 0 {
				break
			}
		}
		offers = append(offers, offersFromItems(p.baseURL, p.cdnURL, raw, opts.Limit)...)
	}
	return offers, nil
}

// handshake fetches page one and mints a session token.
func (p *Parser) handshake(ctx context.Context, endpoint, query string, rule models.Rule) ([]apiItem, string, error) {
	category, isParent := ruleCategoryParams(rule.Category)
	resp, err := p.get(ctx, endpoint, query, 1, "", rule, true, category, isParent)
	if err != nil {
		return nil, "", err
	}
	if resp.Data.SA != "" {
		return resp.Data.Items, resp.Data.SA, nil
	}
	if rule.MinPrice != nil || rule.MaxPrice != nil {
		plain, err := p.get(ctx, endpoint, query, 1, "", rule, false, category, isParent)
		if err != nil {
			return nil, "", err
		}
		if plain.Data.SA != "" {
			return resp.Data.Items, plain.Data.SA, nil
		}
	}
	if resp.Data.Total > 0 && len(resp.Data.Items) == 0 {
		return nil, "", fmt.Errorf("empty sa token")
	}
	return resp.Data.Items, "", nil // zero-result query
}

func (p *Parser) fetchPage(ctx context.Context, endpoint, query string, page int, sa string, rule models.Rule) ([]apiItem, string, error) {
	cat, parent := ruleCategoryParams(rule.Category)
	resp, err := p.get(ctx, endpoint, query, page, sa, rule, true, cat, parent)
	if err != nil {
		return nil, "", err
	}
	return resp.Data.Items, resp.Data.SA, nil
}

func (p *Parser) get(ctx context.Context, endpoint, query string, page int, sa string, rule models.Rule, usePrice bool, category string, isParent bool) (*apiResponse, error) {
	params := url.Values{}
	if query != "" {
		params.Set("search", query)
	}
	params.Set("limit", fmt.Sprintf("%d", pageSize))
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("sort", "desc")
	params.Set("fieldSort", "createdAt")
	if category != "" {
		params.Set("category", category)
	}
	if isParent {
		params.Set("isParentCategory", "1")
	}
	if usePrice {
		if rule.MinPrice != nil {
			params.Set("priceFrom", rule.MinPrice.String())
		}
		if rule.MaxPrice != nil {
			params.Set("priceTo", rule.MaxPrice.String())
		}
	}
	if sa != "" {
		params.Set("sa", sa)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(ctx, req, p.Name())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api status %d", resp.StatusCode)
	}
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &out, nil
}

func offersFromItems(baseURL, cdnURL string, items []apiItem, limit int) []models.Offer {
	out := make([]models.Offer, 0, len(items))
	for _, item := range items {
		offer := toOffer(baseURL, cdnURL, item)
		out = append(out, offer)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func toOffer(baseURL, cdnURL string, item apiItem) models.Offer {
	available := false
	for _, f := range item.Filial {
		if f.Balance > 0 {
			available = true
			break
		}
	}
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
	catTitle := item.Category.Title
	if catTitle == "" {
		catTitle = item.Category.Name
	}
	return models.Offer{
		Key:         item.BarcodeShopUniq,
		Store:       "komissionki",
		Category:    catTitle,
		Title:       item.Name,
		Description: strings.TrimSpace(item.Description),
		URL:         u,
		Price:       price,
		OldPrice:    oldPrice,
		City:        strings.TrimSpace(city),
		Available:   available,
		Images:      imageURLs(cdnURL, item.PhotosResized),
		ParsedAt:    time.Now(),
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
