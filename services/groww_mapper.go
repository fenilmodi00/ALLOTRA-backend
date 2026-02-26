package services

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
)

type GrowwMapper struct {
	utilityService *UtilityService
}

func NewGrowwMapper(utilityService *UtilityService) *GrowwMapper {
	return &GrowwMapper{utilityService: utilityService}
}

func (m *GrowwMapper) MapToIPO(growwData *models.GrowwScrapedIPO, chittItem *ChittorgarhIPOListItem) *models.IPO {
	ipo := &models.IPO{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Slug:      &growwData.Slug,
	}

	if chittItem != nil {
		ipo.StockID = strconv.Itoa(chittItem.ID)
	} else {
		ipo.StockID = "GROWW-" + growwData.Slug
	}

	if growwData.Details != nil {
		d := growwData.Details
		if b, err := json.Marshal(d); err == nil {
			ipo.GrowwDetails = b
		}
		ipo.Name = d.CompanyName
		ipo.Symbol = &d.Symbol
		ipo.Registrar = d.Registrar

		// Map logo URL from Groww
		if d.LogoURL != "" {
			ipo.LogoURL = &d.LogoURL
		}

		if t, err := time.Parse("2006-01-02", d.StartDate); err == nil {
			ipo.OpenDate = &t
		}
		if t, err := time.Parse("2006-01-02", d.EndDate); err == nil {
			ipo.CloseDate = &t
		}
		if t, err := time.Parse("2006-01-02T15:04:05", d.AllotmentDate); err == nil {
			ipo.ResultDate = &t
		}
		if t, err := time.Parse("2006-01-02T15:04:05", d.ListingDate); err == nil {
			ipo.ListingDate = &t
		}

		if d.MinPrice > 0 {
			ipo.PriceBandLow = &d.MinPrice
		}
		if d.MaxPrice > 0 {
			ipo.PriceBandHigh = &d.MaxPrice
		}

		if d.IssueSize > 0 {
			sizeStr := fmt.Sprintf("%.2f Cr", float64(d.IssueSize)/10000000.0)
			ipo.IssueSize = &sizeStr
		}

		if d.MinBidQty > 0 {
			ipo.MinQty = &d.MinBidQty
		}

		if d.AboutCompany.AboutText != "" {
			ipo.About = &d.AboutCompany.AboutText
		}

		if len(d.Pros) > 0 {
			if b, err := json.Marshal(d.Pros); err == nil {
				ipo.Strengths = b
			}
		}
		if len(d.Cons) > 0 {
			if b, err := json.Marshal(d.Cons); err == nil {
				ipo.Risks = b
			}
		}
		if len(d.Financials) > 0 {
			if b, err := json.Marshal(d.Financials); err == nil {
				ipo.Financials = b
			}
		}
		if len(d.Categories) > 0 {
			if b, err := json.Marshal(d.Categories); err == nil {
				ipo.Categories = b
			}
		}
		if len(d.FAQs) > 0 {
			if b, err := json.Marshal(d.FAQs); err == nil {
				ipo.FAQs = b
			}
		}

		ipo.CompanyCode = m.utilityService.GenerateCompanyCode(d.CompanyName)

		// Map Groww's native status as fallback for when dates are missing
		// (e.g. Flipkart, Zepto, PhonePe which have no dates yet).
		// Groww uses "UPCOMING", "ACTIVE", "LISTED" — map to our status constants.
		growwFallback := mapGrowwStatus(d.Status)
		ipo.Status = m.utilityService.CalculateIPOStatus(ipo.OpenDate, ipo.CloseDate, ipo.ListingDate, growwFallback)
	}

	if growwData.CMS != nil && growwData.CMS.Content != "" {
		if ipo.Description == nil {
			ipo.Description = &growwData.CMS.Content
		}
	}

	if growwData.ParsedCMS != nil {
		if b, err := json.Marshal(growwData.ParsedCMS); err == nil {
			ipo.RichData = b
		}
	}

	return ipo
}

// mapGrowwStatus maps Groww's native status strings to our internal status
// constants. Groww uses "UPCOMING", "ACTIVE", "LISTED" etc. in the details API.
func mapGrowwStatus(growwStatus string) string {
	switch growwStatus {
	case "UPCOMING":
		return models.StatusUpcoming
	case "ACTIVE":
		return models.StatusLive
	case "LISTED":
		return models.StatusListed
	default:
		return ""
	}
}
