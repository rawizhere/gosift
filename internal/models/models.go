package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type Rule struct {
	ID        int64
	UserID    int64
	ChatID    int64
	Store     string
	Query     string
	City      string
	MinPrice  *decimal.Decimal
	MaxPrice  *decimal.Decimal
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Offer struct {
	Key         string
	Store       string
	Title       string
	Description string
	URL         string
	Price       decimal.Decimal
	OldPrice    *decimal.Decimal
	City        string
	Available   bool
	Images      []string
	ParsedAt    time.Time
}

type User struct {
	UserID    int64
	Username  string
	FirstName string
	ChatID    int64
	CreatedAt time.Time
}
