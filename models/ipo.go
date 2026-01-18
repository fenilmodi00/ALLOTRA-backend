package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type IPO struct {
	// Primary identification
	ID      uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	StockID string    `json:"stock_id" gorm:"type:varchar(100);not null;uniqueIndex"`

	// Basic Information (from IPOBasicInformation)
	Name        string  `json:"name" gorm:"type:varchar(255);not null"`
	CompanyCode string  `json:"company_code" gorm:"type:varchar(50);not null"`
	Symbol      *string `json:"symbol" gorm:"type:varchar(50)"`
	Registrar   string  `json:"registrar" gorm:"type:varchar(255);not null"`

	// Date Information (from IPODateInformation)
	OpenDate    *time.Time `json:"open_date"`
	CloseDate   *time.Time `json:"close_date"`
	ResultDate  *time.Time `json:"result_date"`
	ListingDate *time.Time `json:"listing_date"`

	// Pricing Information (from IPOPricingInformation)
	PriceBandLow  *float64 `json:"price_band_low" gorm:"type:decimal(10,2)"`
	PriceBandHigh *float64 `json:"price_band_high" gorm:"type:decimal(10,2)"`
	IssueSize     *string  `json:"issue_size" gorm:"type:varchar(100)"`
	MinQty        *int     `json:"min_qty"`
	MinAmount     *int     `json:"min_amount"`

	// Status Information (from IPOStatusInformation)
	Status             string  `json:"status" gorm:"type:varchar(50);not null;default:'Unknown'"`
	SubscriptionStatus *string `json:"subscription_status" gorm:"type:varchar(100)"`
	ListingGain        *string `json:"listing_gain" gorm:"type:varchar(50)"`

	// Additional metadata
	LogoURL     *string `json:"logo_url" gorm:"type:varchar(500)"`
	Description *string `json:"description" gorm:"type:text"`
	About       *string `json:"about" gorm:"type:text"`
	Slug        *string `json:"slug" gorm:"type:varchar(255)"`

	// Legacy form fields (kept for API compatibility)
	FormURL      *string         `json:"form_url" gorm:"type:varchar(500)"`
	FormFields   json.RawMessage `json:"form_fields" gorm:"type:jsonb;default:'{}'"`
	FormHeaders  json.RawMessage `json:"form_headers" gorm:"type:jsonb;default:'{}'"`
	ParserConfig json.RawMessage `json:"parser_config" gorm:"type:jsonb;default:'{}'"`

	// Additional structured data
	Strengths json.RawMessage `json:"strengths" gorm:"type:jsonb;default:'[]'"`
	Risks     json.RawMessage `json:"risks" gorm:"type:jsonb;default:'[]'"`

	// Audit fields
	CreatedAt time.Time `json:"created_at" gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `json:"updated_at" gorm:"default:CURRENT_TIMESTAMP"`
	CreatedBy *string   `json:"created_by" gorm:"type:varchar(100)"`
}
