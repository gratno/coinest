package main

import "time"

type OkexReal struct {
	ID          int64 `gorm:"primary_key;auto_increment"`
	Asks        string
	Bids        string
	Ticker      string
	FundingRate string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
