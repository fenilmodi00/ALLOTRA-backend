// Package services contains the Groww IPO scraper service.
//
// Groww exposes two clean JSON REST APIs (no browser rendering needed):
//
//   - Details API: https://groww.in/v1/api/stocks_primary_market_data/v1/ipo/company/{slug}?isHniEnabled=true
//   - CMS API:     https://cmsapi.groww.in/api/v1/ipo-product-content/{slug}
//
// The scraper uses pure net/http with a shared connection pool, browser-like
// headers, and the existing shared.CircuitBreaker + shared.RetryWithExponentialBackoff
// for resilience. Bulk scraping uses a goroutine worker pool capped at 5 concurrent
// requests to stay polite to Groww's servers.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/sirupsen/logrus"
)

var (
	growwDetailsBaseURL = "https://groww.in/v1/api/stocks_primary_market_data/v1/ipo/company/%s?isHniEnabled=true"
	growwCMSBaseURL     = "https://cmsapi.groww.in/api/v1/ipo-product-content/%s"
	growwDashboardURL   = "https://groww.in/ipo"
	growwIPOPathPrefix  = "/ipo/"
	growwHTTPTimeout    = 30 * time.Second
	growwMaxWorkers     = 5
	growwDataSource     = "groww.in"
)

// GrowwScraperService scrapes IPO data from Groww's internal JSON APIs.
// It is safe for concurrent use; the underlying HTTP client uses a shared
// connection pool with MaxIdleConnsPerHost=10.
type GrowwScraperService struct {
	httpClient     *http.Client
	circuitBreaker *shared.CircuitBreaker
	retryConfig    shared.RetryConfig
	logger         *logrus.Logger
}

// NewGrowwScraperService constructs a GrowwScraperService with production-ready defaults.
// It reuses the project's HTTPClientFactory so connections are pooled across calls.
func NewGrowwScraperService() *GrowwScraperService {
	factory := shared.NewHTTPClientFactory(growwHTTPTimeout)
	client := factory.CreateOptimizedHTTPClient(growwHTTPTimeout)

	cb := shared.NewCircuitBreaker("groww-scraper", shared.DefaultCircuitBreakerConfig())

	return &GrowwScraperService{
		httpClient:     client,
		circuitBreaker: cb,
		retryConfig:    shared.DefaultRetryConfig(),
		logger:         logrus.StandardLogger(),
	}
}

// NewGrowwScraperServiceWithClient constructs a GrowwScraperService with a custom HTTP client.
// This is primarily useful for testing with mock HTTP servers.
func NewGrowwScraperServiceWithClient(client *http.Client, logger *logrus.Logger) *GrowwScraperService {
	cb := shared.NewCircuitBreaker("groww-scraper", shared.DefaultCircuitBreakerConfig())

	return &GrowwScraperService{
		httpClient:     client,
		circuitBreaker: cb,
		retryConfig:    shared.DefaultRetryConfig(),
		logger:         logger,
	}
}

// ─────────────────────────────────────────────
//  Single-IPO fetch methods (Steps 2 & 3)
// ─────────────────────────────────────────────

// ScrapeIPO fetches both the details API and CMS API for a single slug and
// returns a unified GrowwScrapedIPO. Partial results are returned even when
// one of the two APIs fails — the caller can inspect DetailsError / CMSError.
func (s *GrowwScraperService) ScrapeIPO(ctx context.Context, slug string) *models.GrowwScrapedIPO {
	result := &models.GrowwScrapedIPO{
		Slug:       slug,
		ScrapedAt:  time.Now().UTC(),
		DataSource: growwDataSource,
	}

	// Fetch both APIs concurrently; each call is independently protected by
	// the circuit breaker and retry logic.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		details, err := s.FetchIPODetails(ctx, slug)
		if err != nil {
			result.DetailsError = err.Error()
			s.logger.WithFields(logrus.Fields{
				"component": "GrowwScraperService",
				"slug":      slug,
				"error":     err,
			}).Warn("Failed to fetch Groww IPO details")
			return
		}
		result.Details = details
	}()

	go func() {
		defer wg.Done()
		cms, err := s.FetchCMSContent(ctx, slug)
		if err != nil {
			result.CMSError = err.Error()
			s.logger.WithFields(logrus.Fields{
				"component": "GrowwScraperService",
				"slug":      slug,
				"error":     err,
			}).Warn("Failed to fetch Groww CMS content")
			return
		}
		result.CMS = cms
	}()

	wg.Wait()

	if result.CMS != nil && strings.TrimSpace(result.CMS.Content) != "" {
		parsedCMS, err := ParseCMSContent(result.CMS.Content)
		if err != nil {
			s.logger.WithFields(logrus.Fields{
				"component": "GrowwScraperService",
				"slug":      slug,
				"error":     err,
			}).Warn("Failed to parse Groww CMS content")
		} else {
			result.ParsedCMS = parsedCMS
		}
	}

	return result
}

// FetchIPODetails calls the Groww details API for the given slug.
// The call is wrapped in the circuit breaker and retried up to 3 times with
// exponential backoff on transient errors.
func (s *GrowwScraperService) FetchIPODetails(ctx context.Context, slug string) (*models.GrowwIPODetailsResponse, error) {
	url := fmt.Sprintf(growwDetailsBaseURL, slug)

	var response models.GrowwIPODetailsResponse

	err := s.circuitBreaker.Execute(func() error {
		return shared.RetryWithExponentialBackoff(func() error {
			body, fetchErr := s.fetchJSON(ctx, url)
			if fetchErr != nil {
				return fetchErr
			}
			if decodeErr := json.Unmarshal(body, &response); decodeErr != nil {
				return fmt.Errorf("decode IPO details for %s: %w", slug, decodeErr)
			}
			return nil
		}, s.retryConfig, s.logger)
	})

	if err != nil {
		return nil, fmt.Errorf("FetchIPODetails(%s): %w", slug, err)
	}

	s.logger.WithFields(logrus.Fields{
		"component": "GrowwScraperService",
		"slug":      slug,
		"company":   response.CompanyName,
		"status":    response.Status,
	}).Debug("Fetched Groww IPO details")

	return &response, nil
}

// FetchCMSContent calls the Groww CMS API for the given slug.
// Returns a GrowwCMSResponse whose Content field is raw HTML.
// HTTP 404 is treated as a normal "no content" outcome — it is NOT retried
// and does NOT count as a circuit breaker failure, because most IPOs
// legitimately have no CMS content on Groww.
func (s *GrowwScraperService) FetchCMSContent(ctx context.Context, slug string) (*models.GrowwCMSResponse, error) {
	url := fmt.Sprintf(growwCMSBaseURL, slug)

	var response models.GrowwCMSResponse

	err := s.circuitBreaker.Execute(func() error {
		return shared.RetryWithExponentialBackoff(func() error {
			body, fetchErr := s.fetchJSON(ctx, url)
			if fetchErr != nil {
				return fetchErr
			}
			if decodeErr := json.Unmarshal(body, &response); decodeErr != nil {
				return fmt.Errorf("decode CMS content for %s: %w", slug, decodeErr)
			}
			return nil
		}, s.retryConfig, s.logger)
	})

	if err != nil {
		return nil, fmt.Errorf("FetchCMSContent(%s): %w", slug, err)
	}

	// Do not swallow 404 CMS responses; propagate error to caller for CMSError capture

	return &response, nil
}

// fetchJSON executes a GET request with browser-like headers and returns the
// raw response body. The caller is responsible for JSON decoding.
func (s *GrowwScraperService) fetchJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}

	// Mimic a real browser to avoid trivial bot blocks.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://groww.in/ipo")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &shared.HTTPError{StatusCode: resp.StatusCode, URL: url}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body from %s: %w", url, err)
	}

	return body, nil
}

// ─────────────────────────────────────────────
//  Slug discovery (Step 4)
// ─────────────────────────────────────────────

// DiscoverSlugs fetches the Groww IPO dashboard and its sub-pages to parse all
// /ipo/{slug} hrefs. It dedupes slugs found across multiple pages.
// Returns a slice of unique slug strings (e.g. "gaudium-ivf-ipo").
func (s *GrowwScraperService) DiscoverSlugs(ctx context.Context) ([]string, error) {
	s.logger.WithField("component", "GrowwScraperService").Info("Discovering IPO slugs from Groww dashboard and tabs")

	urlsToScrape := []string{
		"https://groww.in/ipo",
		"https://groww.in/ipo/open",
		"https://groww.in/ipo/upcoming",
		"https://groww.in/ipo/closed",
		"https://groww.in/ipo/gmp",
		"https://groww.in/ipo/allotment",
	}

	seen := make(map[string]struct{})
	var slugs []string
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, targetURL := range urlsToScrape {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				s.logger.WithError(err).Warnf("Failed to build request for %s", url)
				return
			}

			// Use HTML Accept header for the dashboard page.
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			req.Header.Set("Accept-Language", "en-US,en;q=0.9")
			req.Header.Set("Cache-Control", "no-cache")

			resp, err := s.httpClient.Do(req)
			if err != nil {
				s.logger.WithError(err).Warnf("Failed to fetch Groww dashboard tab %s", url)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				s.logger.Warnf("Groww dashboard tab %s returned HTTP %d", url, resp.StatusCode)
				return
			}

			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				s.logger.WithError(err).Warnf("Failed to parse HTML from %s", url)
				return
			}

			doc.Find("a[href]").Each(func(_ int, sel *goquery.Selection) {
				href, exists := sel.Attr("href")
				if !exists {
					return
				}

				// Match paths like /ipo/some-company-ipo — must have content after /ipo/
				if !strings.HasPrefix(href, growwIPOPathPrefix) {
					return
				}

				remainder := strings.TrimPrefix(href, growwIPOPathPrefix)
				// Exclude pagination, list, and section URLs
				if remainder == "" || remainder == "open" || remainder == "closed" ||
					remainder == "upcoming" || remainder == "gmp" || remainder == "allotment" ||
					remainder == "sme" || remainder == "mainboard" {
					return
				}

				// Strip any trailing path segments or query strings.
				slug := strings.SplitN(remainder, "/", 2)[0]
				slug = strings.SplitN(slug, "?", 2)[0]
				if slug == "" {
					return
				}

				mu.Lock()
				if _, dup := seen[slug]; !dup {
					seen[slug] = struct{}{}
					slugs = append(slugs, slug)
				}
				mu.Unlock()
			})
		}(targetURL)
	}

	wg.Wait()

	sort.Strings(slugs)

	s.logger.WithFields(logrus.Fields{
		"component":   "GrowwScraperService",
		"slugs_found": len(slugs),
	}).Info("Groww slug discovery complete")

	return slugs, nil
}

// ─────────────────────────────────────────────
//  Concurrent bulk scraping (Step 5)
// ─────────────────────────────────────────────

// ScrapeAll scrapes all provided slugs using a worker pool capped at growwMaxWorkers
// (5) concurrent goroutines. Results preserve input order. Partial failures are
// captured inside each GrowwScrapedIPO.DetailsError / CMSError field.
func (s *GrowwScraperService) ScrapeAll(ctx context.Context, slugs []string) *models.GrowwBulkScrapeResult {
	startedAt := time.Now().UTC()

	result := &models.GrowwBulkScrapeResult{
		StartedAt:       startedAt,
		TotalDiscovered: len(slugs),
		Results:         make([]*models.GrowwScrapedIPO, len(slugs)),
	}

	if len(slugs) == 0 {
		result.CompletedAt = time.Now().UTC()
		result.Duration = time.Since(startedAt).String()
		return result
	}

	type indexedSlug struct {
		index int
		slug  string
	}

	jobs := make(chan indexedSlug, len(slugs))
	for i, slug := range slugs {
		jobs <- indexedSlug{index: i, slug: slug}
	}
	close(jobs)

	var mu sync.Mutex
	var wg sync.WaitGroup

	workerCount := growwMaxWorkers
	if len(slugs) < workerCount {
		workerCount = len(slugs)
	}

	s.logger.WithFields(logrus.Fields{
		"component": "GrowwScraperService",
		"total":     len(slugs),
		"workers":   workerCount,
	}).Info("Starting bulk Groww IPO scrape")

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// Respect context cancellation between items.
				select {
				case <-ctx.Done():
					return
				default:
				}

				scraped := s.ScrapeIPO(ctx, job.slug)

				mu.Lock()
				result.Results[job.index] = scraped
				result.TotalScraped++
				if scraped.DetailsError == "" {
					result.Successful++
				} else {
					result.Failed++
					result.Errors = append(result.Errors,
						fmt.Sprintf("[%s] details: %s", job.slug, scraped.DetailsError))
				}
				mu.Unlock()

				s.logger.WithFields(logrus.Fields{
					"component":      "GrowwScraperService",
					"slug":           job.slug,
					"details_ok":     scraped.DetailsError == "",
					"cms_ok":         scraped.CMSError == "",
					"scraped_so_far": result.TotalScraped,
				}).Debug("Worker completed scrape")
			}
		}()
	}

	wg.Wait()

	result.CompletedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt).String()

	s.logger.WithFields(logrus.Fields{
		"component":  "GrowwScraperService",
		"total":      result.TotalScraped,
		"successful": result.Successful,
		"failed":     result.Failed,
		"duration":   result.Duration,
	}).Info("Bulk Groww IPO scrape complete")

	return result
}

// DiscoverAndScrapeAll is a convenience method that runs slug discovery then
// immediately scrapes all discovered IPOs. Useful for the daily job.
func (s *GrowwScraperService) DiscoverAndScrapeAll(ctx context.Context) (*models.GrowwBulkScrapeResult, error) {
	slugs, err := s.DiscoverSlugs(ctx)
	if err != nil {
		return nil, fmt.Errorf("DiscoverAndScrapeAll discovery: %w", err)
	}

	result := s.ScrapeAll(ctx, slugs)
	return result, nil
}
