// Package models contains data structures for Groww IPO scraper responses.
// Groww exposes two internal JSON APIs used by the scraper:
//   - Details API: https://groww.in/v1/api/stocks_primary_market_data/v1/ipo/company/{slug}?isHniEnabled=true
//   - CMS API:     https://cmsapi.groww.in/api/v1/ipo-product-content/{slug}
package models

import "time"

// GrowwIPODetailsResponse maps the full JSON response from the Groww IPO details API.
// All fields use omitempty to handle partial/upcoming IPO data gracefully.
type GrowwIPODetailsResponse struct {
	// Identity
	CompanyName      string `json:"companyName"`
	CompanyShortName string `json:"growwShortName"`
	Symbol           string `json:"symbol"`
	ISIN             string `json:"isin"`
	IsSME            bool   `json:"isSme"`
	Sector           string `json:"sector"`
	IssueType        string `json:"issueType"` // "EQUITY"
	Status           string `json:"status"`    // "ACTIVE", "UPCOMING", "LISTED"

	// Pricing
	MinPrice   float64 `json:"minPrice"`
	MaxPrice   float64 `json:"maxPrice"`
	IssuePrice float64 `json:"issuePrice"`
	IssueSize  int64   `json:"issueSize"` // in rupees
	LotSize    int     `json:"lotSize"`
	FaceValue  float64 `json:"faceValue"`
	TickSize   float64 `json:"tickSize"`
	MinBidQty  int     `json:"minBidQty"`

	// Schedule
	StartDate     string `json:"startDate"`      // "2026-02-20"
	EndDate       string `json:"endDate"`        // "2026-02-24"
	AllotmentDate string `json:"allotmentDate"`  // "2026-02-25T00:00:00"
	ListingDate   string `json:"listingDate"`    // "2026-02-27T00:00:00"
	DailyStart    string `json:"dailyStartTime"` // "10:00:00"
	DailyEnd      string `json:"dailyEndTime"`   // "17:00:00"
	LastBidTime   string `json:"lastBidPlaceTime"`
	BannerText    string `json:"bannerText"`

	// Subscription
	SubscriptionRates     []GrowwSubscriptionRate `json:"subscriptionRates"`
	SubscriptionUpdatedAt string                  `json:"subscriptionUpdatedAt"`

	// Company profile
	AboutCompany GrowwAboutCompany `json:"aboutCompany"`
	Pros         []string          `json:"pros"`
	Cons         []string          `json:"cons"`

	// Financials (Revenue, Total Assets, Profit)
	Financials []GrowwFinancial `json:"financials"`

	// Listing info
	Listing GrowwListingInfo `json:"listing"`

	// Application categories (IND = Regular, HNI = High Net Worth)
	Categories []GrowwCategory `json:"categories"`

	// Links & media
	DocumentURL        string `json:"documentUrl"`
	LogoURL            string `json:"logoUrl"`
	VideoID            string `json:"videoId"`
	VideoName          string `json:"videoName"`
	RTALink            string `json:"rtaLink"`
	ApplicationDetails string `json:"applicationDetails"`

	// Registrar
	Registrar string `json:"registrar"`

	// Misc
	IsAllotmentAnnounced bool     `json:"isAllotmentAnnounced"`
	PreApplyOpen         bool     `json:"preApplyOpen"`
	CutOffPrice          *float64 `json:"cutOffPrice"`
	ParentSearchID       *string  `json:"parentSearchId"`

	// FAQs
	FAQs []GrowwFAQ `json:"faqs"`
}

// GrowwSubscriptionRate holds per-category subscription rates.
type GrowwSubscriptionRate struct {
	Category     string  `json:"category"`     // "QIB", "NII", "RETAIL", "TOTAL"
	CategoryName string  `json:"categoryName"` // "Qualified Institutional Buyers", etc.
	Rate         float64 `json:"subscriptionRate"`
}

// GrowwAboutCompany contains company profile information.
type GrowwAboutCompany struct {
	YearFounded      string `json:"yearFounded"`
	ManagingDirector string `json:"managingDirector"`
	AboutText        string `json:"aboutCompany"`
}

// GrowwFinancial holds yearly/quarterly financial data for one metric.
type GrowwFinancial struct {
	Title     string             `json:"title"`     // "Revenue", "Total Assets", "Profit"
	Yearly    map[string]float64 `json:"yearly"`    // key: "2023", "2024", "2025"
	Quarterly map[string]float64 `json:"quarterly"` // key: "Q1FY25", etc. (often empty)
}

// GrowwListingInfo contains post-listing exchange data.
type GrowwListingInfo struct {
	ListingPrice *float64 `json:"listingPrice"`
	ListedOn     []string `json:"listedOn"` // ["BSE", "NSE"]
	BSEScripCode *string  `json:"bseScripCode"`
	NSEScripCode *string  `json:"nseScripCode"`
}

// GrowwCategory represents an investor application category (IND / HNI).
type GrowwCategory struct {
	Category         string               `json:"category"`        // "IND", "HNI"
	CategoryLabel    string               `json:"categoryLabel"`   // "Regular", "High Networth Individual"
	CategorySubText  string               `json:"categorySubText"` // "Apply upto ₹2,00,000"
	LotSize          int                  `json:"lotSize"`
	MinBidQuantity   int                  `json:"minBidQuantity"`
	MinPrice         float64              `json:"minPrice"`
	MaxPrice         float64              `json:"maxPrice"`
	SubscriptionRate *float64             `json:"subscriptionRate"`
	State            string               `json:"state"` // "ACTIVE"
	CategoryDiscount float64              `json:"categoryDiscount"`
	IsCategoryActive bool                 `json:"isCategoryActive"`
	CutOffTime       string               `json:"cutOffTime"` // "2026-02-24T16:50:00"
	CategoryDetails  GrowwCategoryDetails `json:"categoryDetails"`
}

// GrowwCategoryDetails holds the user-facing display strings for a category.
type GrowwCategoryDetails struct {
	CategoryLabel string   `json:"categoryLabel"`
	CategoryInfo  []string `json:"categoryInfo"`
}

// GrowwFAQ represents a frequently asked question from Groww's IPO page.
type GrowwFAQ struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// GrowwCMSResponse maps the response from the Groww CMS API.
// Content is raw HTML containing objectives table, registrar info, lead manager, and contact details.
type GrowwCMSResponse struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Content     string `json:"content"` // HTML string
	UpdatedAt   string `json:"updatedAt"`
	PublishedAt string `json:"publishedAt"`
}

// GrowwParsedCMS stores structured fields extracted from CMS HTML content.
type GrowwParsedCMS struct {
	Objectives       []GrowwObjective     `json:"objectives,omitempty"`
	LeadManager      string               `json:"lead_manager,omitempty"`
	RegistrarDetails *GrowwRegistrarInfo  `json:"registrar_details,omitempty"`
	ContactDetails   *GrowwContactDetails `json:"contact_details,omitempty"`
}

// GrowwObjective describes one IPO objective row from CMS content.
type GrowwObjective struct {
	Purpose     string `json:"purpose"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
}

// GrowwRegistrarInfo contains registrar contact details extracted from CMS.
type GrowwRegistrarInfo struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// GrowwContactDetails contains issuer contact details extracted from CMS.
type GrowwContactDetails struct {
	Address string `json:"address"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
}

// GrowwScrapedIPO is the combined, normalized result of scraping both APIs for one IPO.
// This is what the test endpoint returns and what future DB storage will consume.
type GrowwScrapedIPO struct {
	// Discovery metadata
	Slug       string    `json:"slug"`
	ScrapedAt  time.Time `json:"scraped_at"`
	DataSource string    `json:"data_source"` // "groww.in"

	// From details API
	Details *GrowwIPODetailsResponse `json:"details,omitempty"`

	// From CMS API
	CMS *GrowwCMSResponse `json:"cms,omitempty"`

	// Parsed from CMS HTML content
	ParsedCMS *GrowwParsedCMS `json:"parsed_cms,omitempty"`

	// Scrape health
	DetailsError string `json:"details_error,omitempty"`
	CMSError     string `json:"cms_error,omitempty"`
}

// GrowwScrapeTestResult is the response payload for the test endpoint.
// Designed for visual data quality comparison vs Chittorgarh/InvestorGain.
type GrowwScrapeTestResult struct {
	Slug      string           `json:"slug"`
	Success   bool             `json:"success"`
	Duration  string           `json:"duration"`
	ScrapedAt time.Time        `json:"scraped_at"`
	Data      *GrowwScrapedIPO `json:"data,omitempty"`
	Error     string           `json:"error,omitempty"`
}

// GrowwBulkScrapeResult summarises a full discovery + scrape run.
type GrowwBulkScrapeResult struct {
	StartedAt       time.Time          `json:"started_at"`
	CompletedAt     time.Time          `json:"completed_at"`
	Duration        string             `json:"duration"`
	TotalDiscovered int                `json:"total_discovered"`
	TotalScraped    int                `json:"total_scraped"`
	Successful      int                `json:"successful"`
	Failed          int                `json:"failed"`
	Results         []*GrowwScrapedIPO `json:"results"`
	Errors          []string           `json:"errors,omitempty"`
}
