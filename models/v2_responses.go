package models

import "encoding/json"

type V2GMPNested struct {
	Value              *float64 `json:"value"`
	GainPercent        *float64 `json:"gain_percent"`
	EstimatedListing   *float64 `json:"estimated_listing,omitempty"`
	SubscriptionStatus *string  `json:"subscription_status,omitempty"`
}

type V2IPOFeedItem struct {
	ID            string       `json:"id"`
	StockID       string       `json:"stock_id"`
	Name          string       `json:"name"`
	LogoURL       *string      `json:"logo_url"`
	Status        string       `json:"status"`
	Category      string       `json:"category"`
	PriceBandLow  *float64     `json:"price_band_low"`
	PriceBandHigh *float64     `json:"price_band_high"`
	OpenDate      *string      `json:"open_date"`
	CloseDate     *string      `json:"close_date"`
	ListingDate   *string      `json:"listing_date"`
	GMP           *V2GMPNested `json:"gmp,omitempty"`
}

type V2IPODetail struct {
	V2IPOFeedItem
	Description        *string         `json:"description,omitempty"`
	Registrar          string          `json:"registrar"`
	IssueSize          *string         `json:"issue_size,omitempty"`
	MinQty             *int            `json:"min_qty,omitempty"`
	MinAmount          *int            `json:"min_amount,omitempty"`
	SubscriptionStatus *string         `json:"subscription_status,omitempty"`
	Financials         json.RawMessage `json:"financials,omitempty"`
	Categories         json.RawMessage `json:"categories,omitempty"`
	FAQs               json.RawMessage `json:"faqs,omitempty"`
	AllotmentDate      *string         `json:"allotment_date,omitempty"`
	MinInvestment      *float64        `json:"min_investment,omitempty"`
}
