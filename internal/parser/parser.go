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

type Registry struct {
	parsers map[string]Parser
}

func NewRegistry() *Registry {
	return &Registry{parsers: map[string]Parser{}}
}

func (r *Registry) Register(p Parser) error {
	name := p.Name()
	if _, ok := r.parsers[name]; ok {
		return fmt.Errorf("parser %q already registered", name)
	}
	r.parsers[name] = p
	return nil
}

func (r *Registry) Get(name string) (Parser, error) {
	p, ok := r.parsers[name]
	if !ok {
		return nil, fmt.Errorf("parser %q not found", name)
	}
	return p, nil
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.parsers))
	for name := range r.parsers {
		names = append(names, name)
	}
	return names
}

// CategoryPicker returns the store category tree.
func (r *Registry) CategoryPicker(ctx context.Context, store string) ([]models.Category, error) {
	p, err := r.Get(store)
	if err != nil {
		return nil, err
	}
	return p.Categories(ctx)
}
