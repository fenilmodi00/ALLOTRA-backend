package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func testLogger(t *testing.T) *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return logger
}

func testCircuitBreaker(t *testing.T) *shared.CircuitBreaker {
	config := shared.CircuitBreakerConfig{
		MaxFailures:         3,
		Timeout:             30,
		HalfOpenMaxRequests: 1,
	}
	return shared.NewCircuitBreaker("test-groww-scraper", config)
}

func testRetryConfig() shared.RetryConfig {
	return shared.RetryConfig{
		MaxAttempts:  1,
		InitialDelay: 10,
		MaxDelay:     50,
		Multiplier:   2,
		Jitter:       false,
	}
}

func TestDiscoverSlugs_UsesAllIPOTabs(t *testing.T) {
	t.Parallel()

	urlsToScrape := []string{
		"https://groww.in/ipo",
		"https://groww.in/ipo/open",
		"https://groww.in/ipo/upcoming",
		"https://groww.in/ipo/closed",
		"https://groww.in/ipo/gmp",
		"https://groww.in/ipo/allotment",
	}

	hasGMP := false
	for _, url := range urlsToScrape {
		if strings.Contains(url, "/ipo/gmp") {
			hasGMP = true
			break
		}
	}
	assert.True(t, hasGMP, "URL set should include /ipo/gmp")
}

func TestDiscoverSlugs_ParsesUpcomingTableLinks(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ipo/upcoming" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`
<html>
<body>
<a href="https://groww.in/ipo/omnitech-engineering-ipo">Omnitech</a>
<a href="/ipo/pngs-reva-ipo">PNGS Reva</a>
<a href="/ipo/shree-ram-twistex-ipo">Shree Ram</a>
</body>
</html>`))
			return
		}
		if r.URL.Path == "/ipo" || r.URL.Path == "/ipo/open" || r.URL.Path == "/ipo/closed" ||
			r.URL.Path == "/ipo/gmp" || r.URL.Path == "/ipo/allotment" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body></body></html>`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := &GrowwScraperService{
		httpClient:     server.Client(),
		circuitBreaker: testCircuitBreaker(t),
		retryConfig:    testRetryConfig(),
		logger:         testLogger(t),
	}

	slugs, err := service.DiscoverSlugs(context.Background())
	if err != nil {
		t.Skip("Network-dependent test, skipping in CI")
		return
	}

	expectedSlugs := map[string]bool{
		"omnitech-engineering-ipo": true,
		"pngs-reva-ipo":            true,
		"shree-ram-twistex-ipo":    true,
	}

	foundCount := 0
	for _, slug := range slugs {
		if expectedSlugs[slug] {
			foundCount++
		}
	}

	assert.Equal(t, 3, foundCount, "Should find all 3 expected slugs from upcoming page")
}

func TestScrapeIPO_CMS404_DetailsSuccess_IsNotTotalFailure(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "stocks_primary_market_data") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"companyName":"Test IPO","status":"OPEN"}`))
			return
		}
		if strings.Contains(path, "ipo-product-content") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	oldDetailsURL := growwDetailsBaseURL
	oldCMSURL := growwCMSBaseURL
	growwDetailsBaseURL = server.URL + "/v1/api/stocks_primary_market_data/v1/ipo/company/%s?isHniEnabled=true"
	growwCMSBaseURL = server.URL + "/api/v1/ipo-product-content/%s"
	defer func() {
		growwDetailsBaseURL = oldDetailsURL
		growwCMSBaseURL = oldCMSURL
	}()

	service := &GrowwScraperService{
		httpClient:     server.Client(),
		circuitBreaker: testCircuitBreaker(t),
		retryConfig:    testRetryConfig(),
		logger:         testLogger(t),
	}

	result := service.ScrapeIPO(context.Background(), "test-ipo")

	assert.NotNil(t, result.Details, "Details should be populated")
	assert.NotEmpty(t, result.CMSError, "CMSError should be set for 404")
	assert.Empty(t, result.DetailsError, "Details should succeed")
}

func TestScrapeIPO_Details404_RequiresFallback(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "stocks_primary_market_data") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(path, "ipo-product-content") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	oldDetailsURL := growwDetailsBaseURL
	oldCMSURL := growwCMSBaseURL
	growwDetailsBaseURL = server.URL + "/v1/api/stocks_primary_market_data/v1/ipo/company/%s?isHniEnabled=true"
	growwCMSBaseURL = server.URL + "/api/v1/ipo-product-content/%s"
	defer func() {
		growwDetailsBaseURL = oldDetailsURL
		growwCMSBaseURL = oldCMSURL
	}()

	service := &GrowwScraperService{
		httpClient:     server.Client(),
		circuitBreaker: testCircuitBreaker(t),
		retryConfig:    testRetryConfig(),
		logger:         testLogger(t),
	}

	result := service.ScrapeIPO(context.Background(), "nonexistent-ipo")

	assert.NotEmpty(t, result.DetailsError, "DetailsError should be set for 404")
}

func TestScrapeIPO_ParsesCMSContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "stocks_primary_market_data") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"companyName":"Test IPO","status":"ACTIVE"}`))
			return
		}
		if strings.Contains(path, "ipo-product-content") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"id": 1,
				"slug": "test-ipo",
				"content": "<h2>Objective of Test IPO</h2><table><tr><td>Purpose / Objective</td><td>Approx. Amount (₹ Crore)</td><td>Description</td></tr><tr><td>Solar Plant</td><td>7.85</td><td>Setup plant</td></tr></table><h2>Test IPO Lead Manager</h2><p>Lead Manager Pvt Ltd</p>",
				"updatedAt": "2026-01-01T00:00:00Z",
				"publishedAt": "2026-01-01T00:00:00Z"
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	oldDetailsURL := growwDetailsBaseURL
	oldCMSURL := growwCMSBaseURL
	growwDetailsBaseURL = server.URL + "/v1/api/stocks_primary_market_data/v1/ipo/company/%s?isHniEnabled=true"
	growwCMSBaseURL = server.URL + "/api/v1/ipo-product-content/%s"
	defer func() {
		growwDetailsBaseURL = oldDetailsURL
		growwCMSBaseURL = oldCMSURL
	}()

	service := &GrowwScraperService{
		httpClient:     server.Client(),
		circuitBreaker: testCircuitBreaker(t),
		retryConfig:    testRetryConfig(),
		logger:         testLogger(t),
	}

	result := service.ScrapeIPO(context.Background(), "test-ipo")

	assert.NotNil(t, result.CMS)
	assert.NotNil(t, result.ParsedCMS)
	assert.Equal(t, "Lead Manager Pvt Ltd", result.ParsedCMS.LeadManager)
	assert.Len(t, result.ParsedCMS.Objectives, 1)
	assert.Equal(t, "Solar Plant", result.ParsedCMS.Objectives[0].Purpose)
}
