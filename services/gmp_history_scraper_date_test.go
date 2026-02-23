package services

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestParseDateWithStatus_StripsListingSuffix(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{logger: logrus.New()}

	parsed, err := scraper.parseDateWithStatus("21-09-2023 Listing")
	if err != nil {
		t.Fatalf("expected date to parse, got error: %v", err)
	}

	if got := parsed.Format("2006-01-02"); got != "2023-09-21" {
		t.Fatalf("expected 2023-09-21, got %s", got)
	}
}

func TestParseDateWithStatus_StripsLowercaseListingSuffix(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{logger: logrus.New()}

	parsed, err := scraper.parseDateWithStatus("21-09-2023 listing")
	if err != nil {
		t.Fatalf("expected date to parse, got error: %v", err)
	}

	if got := parsed.Format("2006-01-02"); got != "2023-09-21" {
		t.Fatalf("expected 2023-09-21, got %s", got)
	}
}
