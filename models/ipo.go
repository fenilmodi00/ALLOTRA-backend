package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type IPO struct {
	// Primary identification
	ID      uuid.UUID `json:"id" db:"id"`
	StockID string    `json:"stock_id" db:"stock_id"`

	// Basic Information (from IPOBasicInformation)
	Name                 string  `json:"name" db:"name"`
	CompanyCode          string  `json:"company_code" db:"company_code"`
	Symbol               *string `json:"symbol" db:"symbol"`
	Registrar            string  `json:"registrar" db:"registrar"`
	RegistrarID          *string `json:"registrar_id" db:"registrar_id"`
	RegistrarCompanyCode *string `json:"registrar_company_code" db:"registrar_company_code"`
	IsRegistrarFetched   bool    `json:"is_registrar_fetched" db:"is_fetched"`

	// Date Information (from IPODateInformation)
	OpenDate    *time.Time `json:"open_date" db:"open_date"`
	CloseDate   *time.Time `json:"close_date" db:"close_date"`
	ResultDate  *time.Time `json:"result_date" db:"result_date"`
	ListingDate *time.Time `json:"listing_date" db:"listing_date"`

	// Pricing Information (from IPOPricingInformation)
	PriceBandLow  *float64 `json:"price_band_low" db:"price_band_low"`
	PriceBandHigh *float64 `json:"price_band_high" db:"price_band_high"`
	IssueSize     *string  `json:"issue_size" db:"issue_size"`
	MinQty        *int     `json:"min_qty" db:"min_qty"`
	MinAmount     *int     `json:"min_amount" db:"min_amount"`

	// Status Information (from IPOStatusInformation)
	Status             string  `json:"status" db:"status"`
	SubscriptionStatus *string `json:"subscription_status" db:"subscription_status"`
	ListingGain        *string `json:"listing_gain" db:"listing_gain"`

	// Additional metadata
	LogoURL     *string `json:"logo_url" db:"logo_url"`
	Description *string `json:"description" db:"description"`
	About       *string `json:"about" db:"about"`
	Slug        *string `json:"slug" db:"slug"`

	// Legacy form fields (kept for API compatibility)
	FormURL      *string         `json:"form_url" db:"form_url"`
	FormFields   json.RawMessage `json:"form_fields" db:"form_fields"`
	FormHeaders  json.RawMessage `json:"form_headers" db:"form_headers"`
	ParserConfig json.RawMessage `json:"parser_config" db:"parser_config"`

	// Additional structured data
	Strengths json.RawMessage `json:"strengths" db:"strengths"`
	Risks     json.RawMessage `json:"risks" db:"risks"`

	// Additional structured data from Groww
	Financials   json.RawMessage `json:"financials" db:"financials"`
	Categories   json.RawMessage `json:"categories" db:"categories"`
	FAQs         json.RawMessage `json:"faqs" db:"faqs"`
	RichData     json.RawMessage `json:"rich_data" db:"rich_data"`
	GrowwDetails json.RawMessage `json:"groww_details" db:"groww_details"`

	// Audit fields
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	CreatedBy *string   `json:"created_by" db:"created_by"`
}
