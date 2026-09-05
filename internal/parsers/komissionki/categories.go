package komissionki

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rawizhere/gosift/internal/models"
)

// apiCategoryListItem is one node of the store category list.
type apiCategoryListItem struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	Code          string `json:"code"`
	ExtCode       string `json:"extCode"`
	ParentExtCode string `json:"parentExtCode"`
	CountProduct  int    `json:"countProduct"`
}

type apiCategoriesResponse struct {
	Data []apiCategoryListItem `json:"data"`
}

// Categories returns the store category tree.
func (p *Parser) Categories(ctx context.Context) ([]models.Category, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/category", nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(ctx, req, p.Name())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("categories api status %d", resp.StatusCode)
	}
	var out apiCategoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode categories: %w", err)
	}
	hasChildren := make(map[string]bool, len(out.Data))
	for _, c := range out.Data {
		if c.ParentExtCode != "" {
			hasChildren[c.ParentExtCode] = true
		}
	}
	cat := make([]models.Category, 0, len(out.Data))
	for _, c := range out.Data {
		cat = append(cat, models.Category{
			ID:            c.ID,
			Title:         c.Title,
			Code:          c.Code,
			ExtCode:       c.ExtCode,
			ParentExtCode: c.ParentExtCode,
			CountProduct:  c.CountProduct,
			HasChildren:   hasChildren[c.ExtCode],
		})
	}
	return cat, nil
}
