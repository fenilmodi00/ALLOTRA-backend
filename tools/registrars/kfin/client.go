package kfin

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/sirupsen/logrus"
)

const (
	kfinWebBaseURL = "https://ipostatus.kfintech.com"
	kfinAPIBaseURL = "https://0uz601ms56.execute-api.ap-south-1.amazonaws.com/prod/api"
	kfinQueryPath  = "/query"
	kfinTimeout    = 15 * time.Second
)

var (
	kfinIPOScriptPathRegex = regexp.MustCompile(`"(/static/js/main\.[^"]+\.js)"`)
	kfinIPODataRegex       = regexp.MustCompile(`JSON\.parse\('([^']+)'\)`)
)

// shared.AllotmentResult contains detailed allotment data from KFin.
type AllotmentResult struct {
	Status         string // ALLOTTED, NOT_ALLOTTED, NOT_FOUND, ERROR
	ApplicationNo  string
	Name           string
	SharesApplied  int
	SharesAllotted int
	Category       string
}

// KFinAPIResponseItem represents a single record from the KFin API response.
// The API returns an array of these when a match is found.
type KFinAPIResponseItem struct {
	ApplnNo   string `json:"Appln_No"`
	Name      string `json:"Name"`
	PanNo     string `json:"Pan_No"`
	DPCLID    string `json:"DP_CLID"`
	AppShares string `json:"App_Shares"`
	AllShares string `json:"All_Shares"`
	Category  string `json:"category"`
}

// Client handles allotment checking against the KFin (KFintech) registrar.
type Client struct {
	logger *logrus.Entry
}

// NewClient creates a new KFin allotment checker.
func NewClient() *Client {
	return &Client{
		logger: logrus.WithField("component", "kfin_checker"),
	}
}

// newHTTPClient creates an HTTP client with cookie jar for KFin requests.
func (kc *Client) newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: kfinTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar: jar,
	}
}

// CheckAllotmentFromIPO checks allotment using the full IPO model.
// It extracts the KFin company code from FormFields or resolves it
// by matching against the live dropdown on the KFin website.

// extractCompanyCode attempts to get the KFin company code from stored FormFields.

// CheckAllotment checks allotment status on KFin via their AWS API Gateway endpoint.
func (kc *Client) CheckAllotment(ctx context.Context, companyCode string, pan string) (*shared.AllotmentResult, error) {
	client := kc.newHTTPClient()

	// ── Phase 1: Resolve Company Code ────────────────────────────────
	if companyCode == "" {
		kc.logger.Debug("No company code provided, fetching page to find match")
		code, err := kc.findIPOInDropdown(ctx, companyCode)
		if err != nil {
			return nil, fmt.Errorf("could not resolve KFin company code: %w", err)
		}
		companyCode = code
	}

	kc.logger.WithFields(logrus.Fields{
		"company_code": companyCode,
		"pan":          maskPAN(pan),
	}).Debug("Making KFin API request")

	// ── Phase 2: Query the KFin API ──────────────────────────────────
	result, err := kc.queryAPI(ctx, client, "pan", pan, companyCode)
	if err != nil {
		return nil, fmt.Errorf("KFin API query failed: %w", err)
	}

	return result, nil
}

// queryAPI sends a GET request to the KFin AWS API Gateway endpoint.
// searchType is one of: "pan", "appno", "demat"
func (kc *Client) queryAPI(ctx context.Context, client *http.Client, searchType string, searchValue string, companyCode string) (*shared.AllotmentResult, error) {
	// Build the query URL
	params := url.Values{}
	params.Set("type", searchType)
	params.Set("ipocode", companyCode)

	switch searchType {
	case "pan":
		params.Set("pan", strings.ToUpper(searchValue))
	case "appno":
		params.Set("appno", searchValue)
	case "demat":
		params.Set("demat", searchValue)
	default:
		return nil, fmt.Errorf("unsupported search type: %s", searchType)
	}

	apiURL := kfinAPIBaseURL + kfinQueryPath + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	kc.setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("KFin API returned HTTP %d", resp.StatusCode)
	}

	// ── Phase 3: Parse the JSON Response ─────────────────────────────
	var items []KFinAPIResponseItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		// The API may return a string message instead of an array on error
		return &shared.AllotmentResult{Status: "NOT_FOUND"}, nil
	}

	return kc.parseAPIResponse(items), nil
}

// setHeaders sets standard browser-like headers on KFin API requests.
func (kc *Client) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Origin", kfinWebBaseURL)
	req.Header.Set("Referer", kfinWebBaseURL+"/")
}

// parseAPIResponse converts the KFin API response array into our standard result.
func (kc *Client) parseAPIResponse(items []KFinAPIResponseItem) *shared.AllotmentResult {
	result := &shared.AllotmentResult{
		Status: "NOT_FOUND",
	}

	if len(items) == 0 {
		return result
	}

	// Take the first matching record
	item := items[0]

	result.ApplicationNo = strings.TrimSpace(item.ApplnNo)
	result.Name = strings.TrimSpace(item.Name)
	result.SharesApplied = parseIntSafe(item.AppShares)
	result.SharesAllotted = parseIntSafe(item.AllShares)

	if result.SharesAllotted > 0 {
		result.Status = "ALLOTTED"
	} else {
		result.Status = "NOT_ALLOTTED"
	}

	return result
}

// findIPOInDropdown resolves the company code from active IPO options.
func (kc *Client) findIPOInDropdown(ctx context.Context, companyCode string) (string, error) {
	options, err := kc.GetActiveIPOs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch active IPOs: %w", err)
	}
	if len(options) == 0 {
		return "", fmt.Errorf("no IPO options found on KFin for '%s'", companyCode)
	}

	code, score := kc.MatchCompanyName(companyCode, options)
	if code == "" || score <= 0 {
		return "", fmt.Errorf("no matching IPO found for '%s' on KFin", companyCode)
	}

	return code, nil
}

// GetActiveIPOs fetches the list of currently available IPOs on KFin.
func (kc *Client) GetActiveIPOs(ctx context.Context) ([]shared.DropdownOption, error) {
	client := kc.newHTTPClient()
	return kc.getActiveIPOsWithClient(ctx, client)
}

func (kc *Client) getActiveIPOsWithClient(ctx context.Context, client *http.Client) ([]shared.DropdownOption, error) {
	options, apiErr := kc.fetchIPOListFromAPI(ctx, client)
	if apiErr == nil && len(options) > 0 {
		return options, nil
	}
	if apiErr != nil {
		kc.logger.WithError(apiErr).Warn("KFin IPO listing API unavailable, using JS bundle fallback")
	}

	fallbackOptions, fallbackErr := kc.fetchIPOListFromJSBundle(ctx, client)
	if fallbackErr != nil {
		if apiErr != nil {
			return nil, fmt.Errorf("failed to fetch active IPOs from API (%v) and fallback (%w)", apiErr, fallbackErr)
		}
		return nil, fmt.Errorf("failed to fetch active IPOs from fallback: %w", fallbackErr)
	}
	if len(fallbackOptions) == 0 {
		return nil, fmt.Errorf("no active IPOs found in KFin JS fallback")
	}

	return fallbackOptions, nil
}

func (kc *Client) fetchIPOListFromAPI(ctx context.Context, client *http.Client) ([]shared.DropdownOption, error) {
	for _, endpoint := range []string{"/ipolist", "/ipo", "/list"} {
		url := kfinAPIBaseURL + endpoint
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		kc.setHeaders(req)

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		var options []shared.DropdownOption
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return
			}

			var items []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
				return
			}

			options = kc.mapAPIListingItems(items)
		}()

		if len(options) > 0 {
			return options, nil
		}
	}

	return nil, fmt.Errorf("no KFin IPO listing API endpoint returned a valid response")
}

func (kc *Client) mapAPIListingItems(items []map[string]interface{}) []shared.DropdownOption {
	options := make([]shared.DropdownOption, 0, len(items))
	seen := make(map[string]struct{}, len(items))

	for _, item := range items {
		id := firstNonEmptyString(item, "clientId", "ipocode", "ipoCode", "code", "id", "companyCode")
		name := firstNonEmptyString(item, "name", "companyName", "company", "label")
		if id == "" || name == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		options = append(options, shared.DropdownOption{ID: id, Name: name})
	}

	return options
}

// fetchIPOListFromJSBundle is a fragile fallback when no public listing API exists.
// It parses a JSON.parse payload embedded in the current main.*.js bundle.
func (kc *Client) fetchIPOListFromJSBundle(ctx context.Context, client *http.Client) ([]shared.DropdownOption, error) {
	webReq, err := http.NewRequestWithContext(ctx, http.MethodGet, kfinWebBaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create KFin web request: %w", err)
	}
	webReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	webResp, err := client.Do(webReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch KFin web page: %w", err)
	}
	defer webResp.Body.Close()

	if webResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("KFin web page returned HTTP %d", webResp.StatusCode)
	}

	htmlBytes, err := io.ReadAll(webResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read KFin web page: %w", err)
	}

	matches := kfinIPOScriptPathRegex.FindSubmatch(htmlBytes)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not locate main JS bundle in KFin web page")
	}

	jsURL := kfinWebBaseURL + string(matches[1])
	jsReq, err := http.NewRequestWithContext(ctx, http.MethodGet, jsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create JS bundle request: %w", err)
	}
	jsReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	jsResp, err := client.Do(jsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch KFin JS bundle: %w", err)
	}
	defer jsResp.Body.Close()

	if jsResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("KFin JS bundle returned HTTP %d", jsResp.StatusCode)
	}

	jsBytes, err := io.ReadAll(jsResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read KFin JS bundle: %w", err)
	}

	payloadMatch := kfinIPODataRegex.FindSubmatch(jsBytes)
	if len(payloadMatch) < 2 {
		return nil, fmt.Errorf("could not locate embedded IPO payload in KFin JS bundle")
	}

	payload := string(payloadMatch[1])
	var rawItems []struct {
		ClientID string `json:"clientId"`
		Name     string `json:"name"`
	}
	if err := json.Unmarshal([]byte(payload), &rawItems); err != nil {
		unescapedPayload, unescapeErr := strconv.Unquote(`"` + strings.ReplaceAll(payload, `"`, `\\"`) + `"`)
		if unescapeErr != nil {
			return nil, fmt.Errorf("failed to decode IPO payload (%v) and unescape payload (%w)", err, unescapeErr)
		}
		if err := json.Unmarshal([]byte(unescapedPayload), &rawItems); err != nil {
			return nil, fmt.Errorf("failed to decode IPO payload after unescape: %w", err)
		}
	}

	options := make([]shared.DropdownOption, 0, len(rawItems))
	for _, item := range rawItems {
		id := strings.TrimSpace(item.ClientID)
		name := strings.TrimSpace(item.Name)
		if id == "" || name == "" {
			continue
		}
		options = append(options, shared.DropdownOption{ID: id, Name: name})
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no IPO options parsed from KFin JS bundle")
	}

	return options, nil
}

func firstNonEmptyString(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		if str, ok := value.(string); ok {
			trimmed := strings.TrimSpace(str)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

// MatchCompanyName finds the best matching company code from a list of shared dropdown options.
func (kc *Client) MatchCompanyName(companyName string, options []shared.DropdownOption) (string, float64) {
	normalizedTarget := strings.ToLower(strings.TrimSpace(companyName))
	for _, suffix := range []string{" limited", " ltd", " ltd.", " ipo", " sme", " mainboard"} {
		normalizedTarget = strings.TrimSuffix(normalizedTarget, suffix)
	}
	normalizedTarget = strings.TrimSpace(normalizedTarget)

	type match struct {
		code  string
		name  string
		score int
	}
	var bestMatch match

	for _, opt := range options {
		cleanOption := strings.ToLower(strings.TrimSpace(opt.Name))
		for _, suffix := range []string{" limited", " ltd", " ltd.", " ipo", " sme", " - ncd", " - ncds"} {
			cleanOption = strings.TrimSuffix(cleanOption, suffix)
		}
		cleanOption = strings.TrimSpace(cleanOption)

		var score int
		if cleanOption == normalizedTarget {
			score = 10000
		} else if strings.Contains(cleanOption, normalizedTarget) {
			score = 5000 + len(normalizedTarget)
		} else if strings.Contains(normalizedTarget, cleanOption) {
			score = 3000 + len(cleanOption)
		} else {
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
			bestMatch = match{code: opt.ID, name: opt.Name, score: score}
		}
	}

	if bestMatch.code == "" {
		return "", 0
	}

	return bestMatch.code, float64(bestMatch.score)
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
