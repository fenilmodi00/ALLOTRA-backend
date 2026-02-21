package services

import (
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/sirupsen/logrus"
)

func TestValidateScrapedDataMutatesUnderlyingEntries(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{logger: logrus.New()}

	data := &ScrapedHistoryData{
		IPOName:     "Acme IPO GMP",
		IPOPrice:    0,
		CompanyCode: "acme",
		PriceHistory: []models.GMPPriceHistoryEntry{
			{
				RecordDate:       time.Now(),
				IPOPrice:         100,
				GMPValue:         20,
				EstimatedListing: 999,
				ListingPercent:   0,
			},
		},
	}

	if err := scraper.ValidateScrapedData(data); err != nil {
		t.Fatalf("ValidateScrapedData returned error: %v", err)
	}

	if got := data.PriceHistory[0].EstimatedListing; got != 120 {
		t.Fatalf("expected corrected estimated listing 120, got %.2f", got)
	}

	if got := data.PriceHistory[0].ListingPercent; got != 20 {
		t.Fatalf("expected recalculated listing percent 20, got %.2f", got)
	}
}
