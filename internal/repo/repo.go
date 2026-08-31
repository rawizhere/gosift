package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/rawizhere/gosift/internal/models"
)

type Store struct {
	db *sql.DB
	q  *Queries
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, q: New(db)}
}

func (s *Store) CreateUser(ctx context.Context, u models.User) error {
	return s.q.CreateUser(ctx, CreateUserParams{
		UserID:    u.UserID,
		Username:  sql.NullString{String: u.Username, Valid: u.Username != ""},
		FirstName: sql.NullString{String: u.FirstName, Valid: u.FirstName != ""},
		ChatID:    u.ChatID,
	})
}

func (s *Store) CreateRule(ctx context.Context, r models.Rule) error {
	return s.q.CreateRule(ctx, CreateRuleParams{
		UserID:   r.UserID,
		ChatID:   r.ChatID,
		Store:    r.Store,
		Query:    r.Query,
		Category: r.Category,
		City:     r.City,
		MinPrice: decString(r.MinPrice),
		MaxPrice: decString(r.MaxPrice),
	})
}

func (s *Store) ListRulesByUser(ctx context.Context, userID int64) ([]models.Rule, error) {
	rows, err := s.q.ListRulesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toRuleRows(rows), nil
}

func (s *Store) ListEnabledRules(ctx context.Context) ([]models.Rule, error) {
	rows, err := s.q.ListEnabledRules(ctx)
	if err != nil {
		return nil, err
	}
	return toRuleRowsEnabled(rows), nil
}

func (s *Store) GetRule(ctx context.Context, id, userID int64) (models.Rule, error) {
	r, err := s.q.GetRule(ctx, GetRuleParams{ID: id, UserID: userID})
	if err != nil {
		return models.Rule{}, err
	}
	return toRuleRowGet(r), nil
}

func (s *Store) UpdateRule(ctx context.Context, r models.Rule) error {
	return s.q.UpdateRule(ctx, UpdateRuleParams{
		Query:    r.Query,
		Category: r.Category,
		City:     r.City,
		MinPrice: decString(r.MinPrice),
		MaxPrice: decString(r.MaxPrice),
		ID:       r.ID,
		UserID:   r.UserID,
	})
}

func (s *Store) SetRuleEnabled(ctx context.Context, id, userID int64, enabled bool) error {
	return s.q.SetRuleEnabled(ctx, SetRuleEnabledParams{
		Enabled: boolInt(enabled),
		ID:      id,
		UserID:  userID,
	})
}

func (s *Store) DeleteRule(ctx context.Context, id, userID int64) error {
	return s.q.DeleteRule(ctx, DeleteRuleParams{ID: id, UserID: userID})
}

func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	return s.q.GetSetting(ctx, key)
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	return s.q.SetSetting(ctx, SetSettingParams{Key: key, Value: value})
}

func (s *Store) UpsertDialogState(ctx context.Context, chatID int64, state, data string) error {
	return s.q.UpsertDialogState(ctx, UpsertDialogStateParams{
		ChatID: chatID,
		State:  state,
		Data:   sql.NullString{String: data, Valid: data != ""},
	})
}

func (s *Store) GetDialogState(ctx context.Context, chatID int64) (string, string, error) {
	d, err := s.q.GetDialogState(ctx, chatID)
	if err != nil {
		return "", "", err
	}
	return d.State, d.Data.String, nil
}

func (s *Store) DeleteDialogState(ctx context.Context, chatID int64) error {
	return s.q.DeleteDialogState(ctx, chatID)
}

// ShouldNotifyOffer reports whether an offer is new or dropped in price.
func (s *Store) ShouldNotifyOffer(ctx context.Context, chatID int64, key, price string) (bool, error) {
	last, err := s.q.GetSentOffer(ctx, GetSentOfferParams{ChatID: chatID, OfferKey: key})
	if errors.Is(err, sql.ErrNoRows) {
		return true, s.q.InsertSentOffer(ctx, InsertSentOfferParams{
			ChatID:    chatID,
			OfferKey:  key,
			LastPrice: price,
		})
	}
	if err != nil {
		return false, err
	}
	newPrice, err := decimal.NewFromString(price)
	if err != nil {
		return false, fmt.Errorf("parse offer price %q: %w", price, err)
	}
	oldPrice, err := decimal.NewFromString(last)
	if err != nil {
		return false, fmt.Errorf("parse stored price %q: %w", last, err)
	}
	if newPrice.Equal(oldPrice) {
		return false, nil
	}
	if newPrice.GreaterThan(oldPrice) {
		// Price went up: keep following the listing.
		return false, s.q.UpdateSentOfferPrice(ctx, UpdateSentOfferPriceParams{
			LastPrice: price,
			ChatID:    chatID,
			OfferKey:  key,
		})
	}
	// Price dropped: notify and remember the new price.
	if err := s.q.UpdateSentOfferPrice(ctx, UpdateSentOfferPriceParams{
		LastPrice: price,
		ChatID:    chatID,
		OfferKey:  key,
	}); err != nil {
		return false, err
	}
	return true, nil
}

func decString(d *decimal.Decimal) sql.NullString {
	if d == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: d.String(), Valid: true}
}

func boolInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

func toRuleRows(rows []ListRulesByUserRow) []models.Rule {
	out := make([]models.Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, toRuleRow(ruleRowFromList(r)))
	}
	return out
}

// toRuleRowsEnabled converts the enabled-rules rows.
func toRuleRowsEnabled(rows []ListEnabledRulesRow) []models.Rule {
	out := make([]models.Rule, 0, len(rows))
	for _, r := range rows {
		out = append(out, toRuleRow(ruleRowFromEnabled(r)))
	}
	return out
}

func toRuleRowGet(r GetRuleRow) models.Rule {
	return toRuleRow(ruleRowFromGet(r))
}

// ruleRow is the shape shared by the sqlc rules rows.
type ruleRow struct {
	ID        int64
	UserID    int64
	ChatID    int64
	Store     string
	Query     string
	Category  string
	City      string
	MinPrice  sql.NullString
	MaxPrice  sql.NullString
	Enabled   int64
	CreatedAt string
	UpdatedAt string
}

func ruleRowFromList(r ListRulesByUserRow) ruleRow {
	return ruleRow(r)
}

func ruleRowFromEnabled(r ListEnabledRulesRow) ruleRow {
	return ruleRow(r)
}

func ruleRowFromGet(r GetRuleRow) ruleRow {
	return ruleRow(r)
}

func toRuleRow(r ruleRow) models.Rule {
	return models.Rule{
		ID:        r.ID,
		UserID:    r.UserID,
		ChatID:    r.ChatID,
		Store:     r.Store,
		Query:     r.Query,
		Category:  r.Category,
		City:      r.City,
		MinPrice:  toDec(r.MinPrice),
		MaxPrice:  toDec(r.MaxPrice),
		Enabled:   r.Enabled == 1,
		CreatedAt: timeParse(r.CreatedAt),
		UpdatedAt: timeParse(r.UpdatedAt),
	}
}

func toDec(s sql.NullString) *decimal.Decimal {
	if !s.Valid || s.String == "" {
		return nil
	}
	d, err := decimal.NewFromString(s.String)
	if err != nil {
		return nil
	}
	return &d
}
