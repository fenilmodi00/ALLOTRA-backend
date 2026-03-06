package bigshare

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/sirupsen/logrus"
)

const (
	bigshareBaseURL = "https://ipo.bigshareonline.com"
	bigsharePage    = "/ipo_status.html"
	bigshareAPI     = "/Data.aspx/FetchIpodetails"
	bigshareTimeout = 15 * time.Second
)

// BigshareAPIRequest matches the AJAX payload sent by the Bigshare site
type BigshareAPIRequest struct {
	ApplicationNo string `json:"Applicationno"`
	Company       string `json:"Company"`       // The dropdown ID
	SelectionType string `json:"SelectionType"` // "PN" for PAN
	PanNo         string `json:"PanNo"`
	TxtCsdl       string `json:"txtcsdl"`
	TxtDPID       string `json:"txtDPID"`
	TxtClId       string `json:"txtClId"`
	DdlType       string `json:"ddlType"`
	Lang          string `json:"lang"` // "en"
}

// BigshareAPIResponse matches the JSON structure returned by the Bigshare API
type BigshareAPIResponse struct {
	D struct {
		ApplicationNo string `json:"APPLICATION_NO"`
		DPID          string `json:"DPID"` // Can also contain error messages like "No data found"
		Name          string `json:"Name"`
		Applied       string `json:"APPLIED"`
		Allotted      string `json:"ALLOTED"`
	} `json:"d"`
}

// Client handles allotment checking against the Bigshare registrar.
type Client struct {
	logger *logrus.Entry
}

// NewClient creates a new Bigshare allotment checker
func NewClient() *Client {
	return &Client{
		logger: logrus.WithField("component", "bigshare_checker"),
	}
}

// newHTTPClient creates an HTTP client with cookie jar
func (bc *Client) newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: bigshareTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar: jar,
	}
}

// setHeaders sets standard browser headers on the request
func (bc *Client) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://ipo.bigshareonline.com")
	req.Header.Set("Referer", "https://ipo.bigshareonline.com/ipo_status.html")
}

// CheckAllotmentFromIPO checks allotment using the full IPO model.

// CheckAllotment checks allotment status on Bigshare via their new JSON API (Data.aspx)
func (bc *Client) CheckAllotment(ctx context.Context, companyCode string, pan string) (*shared.AllotmentResult, error) {
	client := bc.newHTTPClient()

	// ── Phase 1: Determine Dropdown ID ───────────────────────────────
	// If we don't know the exact ID, we must fetch the main page and find it
	if companyCode == "" {
		bc.logger.Debug("No dropdown ID provided, fetching page to find match")
		val := companyCode
		var err error
		if err != nil {
			return nil, fmt.Errorf("could not resolve IPO ID: %w", err)
		}
		companyCode = val
	}

	bc.logger.WithFields(logrus.Fields{
		"company_id": companyCode,
		"pan":        maskPAN(pan),
	}).Debug("Making API request")

	// ── Phase 2: Send JSON API Request ────────────────────────────────
	payload := BigshareAPIRequest{
		ApplicationNo: "",
		Company:       companyCode,
		SelectionType: "PN",
		PanNo:         pan,
		TxtCsdl:       "",
		TxtDPID:       "",
		TxtClId:       "",
		DdlType:       "",
		Lang:          "en",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	apiURL := bigshareBaseURL + bigshareAPI
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %w", err)
	}
	bc.setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned HTTP %d", resp.StatusCode)
	}

	// ── Phase 3: Parse JSON Response ──────────────────────────────────
	var apiResp BigshareAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	return bc.parseAPIResponse(&apiResp), nil
}

// parseAPIResponse converts the Bigshare JSON response into our standard format
func (bc *Client) parseAPIResponse(resp *BigshareAPIResponse) *shared.AllotmentResult {
	result := &shared.AllotmentResult{
		Status: shared.StatusNotFound,
	}

	// Check for standard error strings in the DPID field (Bigshare weirdness)
	errorMsg := strings.ToLower(resp.D.DPID)
	if errorMsg == "no data found" || strings.Contains(errorMsg, "invalid") || strings.Contains(errorMsg, "please enter valid") {
		return result // Returns NOT_FOUND
	}

	// If we have an application number, we have a successful fetch
	if resp.D.ApplicationNo != "" {
		result.ApplicationNo = resp.D.ApplicationNo
		result.SharesApplied = parseIntSafe(resp.D.Applied)
		result.SharesAllotted = parseIntSafe(resp.D.Allotted)

		if result.SharesAllotted > 0 {
			result.Status = shared.StatusAllotted
		} else {
			result.Status = shared.StatusNotAllotted
		}
	}

	return result
}

// MatchCompanyName finds the best matching company ID from a list of options based on the company name
func (bc *Client) MatchCompanyName(companyName string, options []shared.DropdownOption) (string, float64) {
	normalizedTarget := strings.ToLower(strings.TrimSpace(companyName))
	for _, suffix := range []string{" limited", " ltd", " ltd.", " ipo", " sme", " mainboard"} {
		normalizedTarget = strings.TrimSuffix(normalizedTarget, suffix)
	}
	normalizedTarget = strings.TrimSpace(normalizedTarget)

	type match struct {
		value string
		text  string
		score int
	}
	var bestMatch match

	for _, opt := range options {
		cleanOption := strings.ToLower(opt.Name)
		for _, suffix := range []string{" limited", " ltd", " ltd.", " ipo", " sme"} {
			cleanOption = strings.TrimSuffix(cleanOption, suffix)
		}
		cleanOption = strings.TrimSpace(cleanOption)

		var score int
		if cleanOption == normalizedTarget {
			score = 10000 // Exact match
		} else if strings.Contains(cleanOption, normalizedTarget) {
			score = 5000 + len(normalizedTarget) // Exact substring
		} else if strings.Contains(normalizedTarget, cleanOption) {
			score = 3000 + len(cleanOption) // Option in target
		} else {
			// Word overlap
			targetWords := strings.Fields(normalizedTarget)
			matchedWords := 0
			for _, word := range targetWords {
				if len(word) > 2 && strings.Contains(cleanOption, word) {
					matchedWords++
				}
			}
			if matchedWords > 0 && matchedWords >= len(targetWords)/2 {
				score = matchedWords * 100
			}
		}

		if score > bestMatch.score {
			bestMatch = match{value: opt.ID, text: opt.Name, score: score}
		}
	}

	if bestMatch.value == "" {
		return "", 0
	}

	return bestMatch.value, float64(bestMatch.score)
}

// findIPOInLiveDropdown fetches the HTML page and searches the dropdown for the company name
func (bc *Client) findIPOInLiveDropdown(ctx context.Context, client *http.Client, companyName string) (string, error) {
	pageURL := bigshareBaseURL + bigsharePage
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Setup matching logic
	normalizedTarget := strings.ToLower(strings.TrimSpace(companyName))
	for _, suffix := range []string{" limited", " ltd", " ltd.", " ipo", " sme", " mainboard"} {
		normalizedTarget = strings.TrimSuffix(normalizedTarget, suffix)
	}
	normalizedTarget = strings.TrimSpace(normalizedTarget)

	type match struct {
		value string
		text  string
		score int
	}
	var bestMatch match

	doc.Find("select#ddlCompany option").Each(func(i int, s *goquery.Selection) {
		val, exists := s.Attr("value")
		text := strings.TrimSpace(s.Text())
		if !exists || val == "0" || val == "" || strings.Contains(strings.ToLower(text), "select company") {
			return
		}

		cleanOption := strings.ToLower(text)
		for _, suffix := range []string{" limited", " ltd", " ltd.", " ipo", " sme"} {
			cleanOption = strings.TrimSuffix(cleanOption, suffix)
		}
		cleanOption = strings.TrimSpace(cleanOption)

		var score int
		if cleanOption == normalizedTarget {
			score = 10000 // Exact match
		} else if strings.Contains(cleanOption, normalizedTarget) {
			score = 5000 + len(normalizedTarget) // Exact substring
		} else if strings.Contains(normalizedTarget, cleanOption) {
			score = 3000 + len(cleanOption) // Option in target
		} else {
			// Word overlap
			targetWords := strings.Fields(normalizedTarget)
			matchedWords := 0
			for _, word := range targetWords {
				if len(word) > 2 && strings.Contains(cleanOption, word) {
					matchedWords++
				}
			}
			if matchedWords > 0 && matchedWords >= len(targetWords)/2 {
				score = matchedWords * 100
			}
		}

		if score > bestMatch.score {
			bestMatch = match{value: val, text: text, score: score}
		}
	})

	if bestMatch.value == "" {
		return "", fmt.Errorf("no matching IPO found for '%s'", companyName)
	}

	return bestMatch.value, nil
}

// GetActiveIPOs fetches the list of currently available IPOs
func (bc *Client) GetActiveIPOs(ctx context.Context) ([]shared.DropdownOption, error) {
	client := bc.newHTTPClient()
	pageURL := bigshareBaseURL + bigsharePage

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var options []shared.DropdownOption
	doc.Find("select#ddlCompany option").Each(func(i int, s *goquery.Selection) {
		val, exists := s.Attr("value")
		text := strings.TrimSpace(s.Text())
		if exists && val != "0" && val != "" && !strings.Contains(strings.ToLower(text), "select company") {
			options = append(options, shared.DropdownOption{ID: val, Name: text})
		}
	})

	return options, nil
}

// BigshareIPOOption represents an IPO option from the dropdown
type BigshareIPOOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func parseIntSafe(s string) int {
	var result int
	clean := strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	fmt.Sscanf(clean, "%d", &result)
	return result
}

func maskPAN(pan string) string {
	if len(pan) >= 4 {
		return pan[:3] + "****" + pan[len(pan)-1:]
	}
	return pan
}
