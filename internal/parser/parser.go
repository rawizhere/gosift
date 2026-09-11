package parser

import (
	"context"
	"fmt"

	"github.com/rawizhere/gosift/internal/models"
)

type SearchOptions struct {
	Limit int
	Query string
}

type Parser interface {
	Name() string
	Search(ctx context.Context, rule models.Rule, opts SearchOptions) ([]models.Offer, error)
	Categories(ctx context.Context) ([]models.Category, error)
}

// Registry maps store names to their parsers.
type Registry map[string]Parser

func (r Registry) Get(name string) (Parser, error) {
	p, ok := r[name]
	if !ok {
		return nil, fmt.Errorf("parser %q not found", name)
	}
	return p, nil
}

func (r Registry) Names() []string {
	names := make([]string, 0, len(r))
	for name := range r {
		names = append(names, name)
	}
	return names
}

// Categories returns the category tree of the given store.
func (r Registry) Categories(ctx context.Context, store string) ([]models.Category, error) {
	p, err := r.Get(store)
	if err != nil {
		return nil, err
	}
	return p.Categories(ctx)
}
