package models

import "github.com/shopspring/decimal"

type Rule struct {
	ID       int64
	UserID   int64
	ChatID   int64
	Store    string
	Query    string
	Category string
	City     string
	MinPrice *decimal.Decimal
	MaxPrice *decimal.Decimal
	Enabled  bool
}

type Offer struct {
	Key         string
	Store       string
	Category    string
	Title       string
	Description string
	URL         string
	Price       decimal.Decimal
	OldPrice    *decimal.Decimal
	City        string
	Images      []string
}

type User struct {
	UserID    int64
	Username  string
	FirstName string
	ChatID    int64
}

// Category is a store category node; ParentExtCode links a child to its parent.
type Category struct {
	ID            int64
	Title         string
	ExtCode       string
	ParentExtCode string
	HasChildren   bool
}
