package komissionki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
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
	Price           json.Number  `json:"price"`
	OldPrice        *json.Number `json:"oldPrice"`
	City            []apiCity    `json:"city"`
	Filial          []apiFilial  `json:"filial"`
	Category        apiCategory  `json:"category"`
	ParentCategory  apiCategory  `json:"parentCategory"`
}

type apiCity struct {
	Name string `json:"name"`
}

type apiFilial struct {
	Balance int `json:"balance"`
}

type apiCategory struct {
	Code string `json:"code"`
}

type Parser struct {
	client  *httpclient.Client
	baseURL string
	apiURL  string
}

func New(client *httpclient.Client, baseURL, apiURL string) *Parser {
	return &Parser{client: client, baseURL: baseURL, apiURL: apiURL}
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
	sa, err := p.handshake(ctx, endpoint, query, rule)
	if err != nil {
		return nil, fmt.Errorf("handshake: %w", err)
	}
	var offers []models.Offer
	for page := 1; len(offers) < opts.Limit; page++ {
		raw, nextSA, err := p.fetchPage(ctx, endpoint, query, page, sa, rule)
		if err != nil {
			if !errors.Is(err, errUnauthorized) {
				return nil, err
			}
			sa, err = p.handshake(ctx, endpoint, query, rule)
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
			reSA, err := p.handshake(ctx, endpoint, query, rule)
			if err != nil {
				break
			}
			raw, nextSA, err = p.fetchPage(ctx, endpoint, query, page, reSA, rule)
			if err != nil {
				return nil, err
			}
			sa = nextSA
			if len(raw) == 0 {
				break
			}
		}
		for _, item := range raw {
			offers = append(offers, toOffer(p.baseURL, item))
			if len(offers) >= opts.Limit {
				break
			}
		}
	}
	return offers, nil
}

func (p *Parser) handshake(ctx context.Context, endpoint, query string, rule models.Rule) (string, error) {
	resp, err := p.get(ctx, endpoint, query, 1, "", rule)
	if err != nil {
		return "", err
	}
	if resp.Data.SA == "" {
		return "", fmt.Errorf("empty sa token")
	}
	return resp.Data.SA, nil
}

func (p *Parser) fetchPage(ctx context.Context, endpoint, query string, page int, sa string, rule models.Rule) ([]apiItem, string, error) {
	resp, err := p.get(ctx, endpoint, query, page, sa, rule)
	if err != nil {
		return nil, "", err
	}
	return resp.Data.Items, resp.Data.SA, nil
}

func (p *Parser) get(ctx context.Context, endpoint, query string, page int, sa string, rule models.Rule) (*apiResponse, error) {
	params := url.Values{}
	params.Set("search", query)
	params.Set("limit", fmt.Sprintf("%d", pageSize))
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("sort", "desc")
	params.Set("fieldSort", "createdAt")
	if rule.MinPrice != nil {
		params.Set("priceFrom", rule.MinPrice.String())
	}
	if rule.MaxPrice != nil {
		params.Set("priceTo", rule.MaxPrice.String())
	}
	if sa != "" {
		params.Set("sa", sa)
	}
	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
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

func toOffer(baseURL string, item apiItem) models.Offer {
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
	return models.Offer{
		Key:       item.BarcodeShopUniq,
		Store:     "komissionki",
		Title:     item.Name,
		URL:       u,
		Price:     price,
		OldPrice:  oldPrice,
		City:      strings.TrimSpace(city),
		Available: available,
		ParsedAt:  time.Now(),
	}
}
