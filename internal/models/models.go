package models

import "time"

type Rule struct {
	ID        int64
	UserID    int64
	ChatID    int64
	Store     string
	Query     string
	City      string
	MinPrice  string
	MaxPrice  string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Offer struct {
	Store     string
	Title     string
	URL       string
	Price     string
	OldPrice  string
	City      string
	Available bool
	ParsedAt  time.Time
}

type User struct {
	UserID    int64
	Username  string
	FirstName string
	ChatID    int64
	CreatedAt time.Time
}
