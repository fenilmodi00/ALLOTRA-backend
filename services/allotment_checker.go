package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
)

// AllotmentChecker handles checking IPO allotment status
type AllotmentChecker struct {
	RateLimiter *shared.HTTPRequestRateLimiter
}

// NewAllotmentChecker creates a new allotment checker
func NewAllotmentChecker() *AllotmentChecker {
	return &AllotmentChecker{
		RateLimiter: shared.NewHTTPRequestRateLimiter(2 * time.Second), // More conservative rate limiting for allotment checks
	}
}

// CheckAllotmentStatus checks the allotment status for a given IPO and PAN
func (a *AllotmentChecker) CheckAllotmentStatus(ctx context.Context, ipo *models.IPO, pan string) (string, int, error) {
	// Apply rate limiting for politeness
	a.RateLimiter.EnforceRateLimit()

	// 1. Parse Configs
	var formFields map[string]string
	if err := json.Unmarshal(ipo.FormFields, &formFields); err != nil {
		return "", 0, fmt.Errorf("invalid form fields config: %w", err)
	}

	var formHeaders map[string]string
	if err := json.Unmarshal(ipo.FormHeaders, &formHeaders); err != nil {
		return "", 0, fmt.Errorf("invalid form headers config: %w", err)
	}

	type ParserConfig struct {
		SubmitURL       string `json:"submit_url"` // Optional override
		StatusSelectors struct {
			Allotted    []string `json:"allotted"`
			NotAllotted []string `json:"not_allotted"`
		} `json:"status_selectors"`
	}
	var parserConfig ParserConfig
	if err := json.Unmarshal(ipo.ParserConfig, &parserConfig); err != nil {
		return "", 0, fmt.Errorf("invalid parser config: %w", err)
	}

	// 2. Initialize Collector (Single instance to maintain session)
	c := colly.NewCollector()

	// Set Headers Global
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		if ipo.FormURL != nil {
			r.Headers.Set("Referer", *ipo.FormURL)
		}
		for k, v := range formHeaders {
			r.Headers.Set(k, v)
		}
		// Ensure Content-Type is set for POST requests if not already in formHeaders
		if r.Method == "POST" && r.Headers.Get("Content-Type") == "" {
			r.Headers.Set("Content-Type", "application/json; charset=utf-8")
		}
		// Add X-Requested-With for AJAX calls
		r.Headers.Set("X-Requested-With", "XMLHttpRequest")

		logrus.Infof("Requesting %s %s with Headers: %v", r.Method, r.URL, r.Headers)
	})

	// 3. Scrape Hidden Fields (if any)
	scrapedData := make(map[string]string)
	needsScraping := false
	for _, v := range formFields {
		if len(v) > 7 && v[:7] == "SCRAPE:" {
			needsScraping = true
			break
		}
	}

	if needsScraping {
		c.OnHTML("html", func(e *colly.HTMLElement) {
			for k, v := range formFields {
				if len(v) > 7 && v[:7] == "SCRAPE:" {
					selector := v[7:]
					val, _ := e.DOM.Find(selector).Attr("value")
					scrapedData[k] = val
				}
			}
		})
		if ipo.FormURL != nil {
			if err := c.Visit(*ipo.FormURL); err != nil {
				return "", 0, fmt.Errorf("failed to scrape form page: %w", err)
			}
		} else {
			return "", 0, fmt.Errorf("IPO FormURL is nil, cannot scrape form page")
		}
	}

	// 4. Prepare Payload
	logrus.Infof("Scraped Data: %v", scrapedData)
	data := make(map[string]interface{})
	for k, v := range formFields {
		if v == "USER_INPUT" {
			data[k] = pan
		} else if len(v) > 7 && v[:7] == "SCRAPE:" {
			if val, ok := scrapedData[k]; ok && val != "" {
				data[k] = val
			} else if k == "token" && scrapedData["token"] != "" {
				data[k] = scrapedData["token"]
			} else {
				data[k] = ""
			}

			// Hack/Fallback for CHKVAL if empty
			if k == "CHKVAL" && (data[k] == "" || data[k] == nil) {
				logrus.Warn("CHKVAL is empty, defaulting to '1'")
				data[k] = "1"
			}
		} else {
			data[k] = v
		}
	}
	logrus.Infof("Final Payload Keys: %v", a.reflectKeys(data))

	// 5. Execute Request
	targetURL := ipo.FormURL
	if parserConfig.SubmitURL != "" {
		targetURL = &parserConfig.SubmitURL
	}

	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal payload: %w", err)
	}
	logrus.Infof("Final JSON Payload: %s", string(jsonPayload))

	var status string = "NOT_FOUND"
	var shares int = 0

	var errorBody string
	// Log Error Response
	c.OnError(func(r *colly.Response, err error) {
		errorBody = string(r.Body)
		logrus.Errorf("Scraper Error: %v, Body: %s", err, errorBody)
	})

	// Parse Response (Handle JSON response if Content-Type is JSON)
	c.OnResponse(func(r *colly.Response) {
		if len(r.Body) > 0 && (r.Headers.Get("Content-Type") == "application/json" || r.Headers.Get("content-type") == "application/json; charset=utf-8") {
			// Try to parse JSON response
			var resp map[string]interface{}
			if err := json.Unmarshal(r.Body, &resp); err == nil {
				if d, ok := resp["d"].(string); ok {
					// Parse HTML in 'd'
					doc, err := goquery.NewDocumentFromReader(strings.NewReader(d))
					if err != nil {
						logrus.Errorf("Failed to parse HTML in response: %v", err)
						return
					}

					// Check Allotted
					for _, selector := range parserConfig.StatusSelectors.Allotted {
						if doc.Find(selector).Length() > 0 {
							status = "ALLOTTED"
							// Extract shares if possible (assuming standard table structure or selector)
							// For now, just set status
							break
						}
					}
					// Check Not Allotted
					if status == "NOT_FOUND" {
						for _, selector := range parserConfig.StatusSelectors.NotAllotted {
							if doc.Find(selector).Length() > 0 {
								status = "NOT_ALLOTTED"
								break
							}
						}
					}

					// If still not found, log the HTML for debugging
					if status == "NOT_FOUND" {
						logrus.Warnf("Status not found in response HTML: %s", d)
					}
				}
			}
		}
	})

	// Fallback HTML parsing
	c.OnHTML("html", func(e *colly.HTMLElement) {
		// Check Allotted
		for _, selector := range parserConfig.StatusSelectors.Allotted {
			if e.DOM.Find(selector).Length() > 0 {
				status = "ALLOTTED"
				return
			}
		}
		// Check Not Allotted
		for _, selector := range parserConfig.StatusSelectors.NotAllotted {
			if e.DOM.Find(selector).Length() > 0 {
				status = "NOT_ALLOTTED"
				return
			}
		}
	})

	if targetURL == nil {
		return "", 0, fmt.Errorf("target URL is nil, cannot make request")
	}

	err = c.PostRaw(*targetURL, jsonPayload)
	if err != nil {
		// The error might be from OnError, so we check if we got a status
		if status != "NOT_FOUND" {
			return status, shares, nil
		}
		return "", 0, fmt.Errorf("failed to post to registrar: %w, Body: %s", err, errorBody)
	}

	return status, shares, nil
}

// reflectKeys returns the keys of a map
func (a *AllotmentChecker) reflectKeys(data map[string]interface{}) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	return keys
}
