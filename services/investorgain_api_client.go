package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	InvestorGainBaseURL = "https://webnodejs.investorgain.com/cloud/new/ipo"
)

var ErrNoGMPDataAvailable = errors.New("no GMP data available")

type InvestorGainAPIClient struct {
	httpClient *http.Client
	logger     *logrus.Logger
}

func NewInvestorGainAPIClient() *InvestorGainAPIClient {
	return &InvestorGainAPIClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logrus.New(),
	}
}

type IPOUrlListResponse struct {
	Data  []IPOUrlEntry  `json:"data"`
	Lists []IPOURLListV2 `json:"lists"`
}

type IPOURLListV2 struct {
	ID                   int    `json:"id"`
	CompanyCode          string `json:"company_code"`
	CompanyName          string `json:"company_name"`
	CompanyShortName     string `json:"company_short_name"`
	URL                  string `json:"url"`
	URLRewriteFolderName string `json:"urlrewrite_folder_name"`
}

type IPOUrlEntry struct {
	CompanyCode string `json:"company_code"`
	CompanyName string `json:"company_name"`
	URL         string `json:"url"`
	URLCode     string `json:"url_code,omitempty"`
	NumericID   string `json:"numeric_id,omitempty"`
}

type IPOGMPResponse struct {
	Msg         int             `json:"msg"`
	Key         string          `json:"key"`
	CurrentTime string          `json:"currentTime"`
	IpoGmpTable string          `json:"ipoGmpTable"`
	IpoGmpData  json.RawMessage `json:"ipoGmpData"`
}

type IPOGMPPayload struct {
	Key         string
	CurrentTime string
	TableHTML   string
	DataPoints  []IPOGmpDataPoint
}

type FlexibleString string

func (s *FlexibleString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*s = ""
		return nil
	}

	if trimmed[0] == '"' {
		var str string
		if err := json.Unmarshal(trimmed, &str); err != nil {
			return err
		}
		*s = FlexibleString(strings.TrimSpace(str))
		return nil
	}

	*s = FlexibleString(strings.TrimSpace(string(trimmed)))
	return nil
}

func (s FlexibleString) String() string {
	return strings.TrimSpace(string(s))
}

type IPOGmpDataPoint struct {
	Date                  FlexibleString `json:"gmp_date"`
	LegacyDate            FlexibleString `json:"date"`
	GMP                   FlexibleString `json:"gmp"`
	LegacyGMP             FlexibleString `json:"gmp_value"`
	IPOPrice              FlexibleString `json:"max_ipo_price"`
	LegacyIPOPrice        FlexibleString `json:"ipo_price"`
	EstimatedListing      FlexibleString `json:"estimated_listing_price"`
	LegacyEstimated       FlexibleString `json:"estimated_listing"`
	EstimatedPercent      FlexibleString `json:"gmp_percent_calc"`
	LegacyEstimatedPct    FlexibleString `json:"estimated_percent"`
	Sub2                  FlexibleString `json:"sub2"`
	LegacySub2            FlexibleString `json:"subject_to_sauda"`
	EstimatedProfit       FlexibleString `json:"est_profit"`
	LegacyEstimatedProfit FlexibleString `json:"estimated_profit"`
	LastUpdated           FlexibleString `json:"last_updated"`
	LegacyLastUpdated     FlexibleString `json:"last_updated_gmp"`
}

func (c *InvestorGainAPIClient) GetIPOUrlList() ([]IPOUrlEntry, error) {
	url := fmt.Sprintf("%s/ipo-url-lists", InvestorGainBaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result IPOUrlListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) > 0 {
		return normalizeIPOUrlEntries(result.Data), nil
	}

	if len(result.Lists) == 0 {
		return []IPOUrlEntry{}, nil
	}

	entries := make([]IPOUrlEntry, 0, len(result.Lists))
	for _, item := range result.Lists {
		companyCode := normalizeCompanyCode(item.CompanyCode)
		urlCode := normalizeCompanyCode(item.URLRewriteFolderName)

		numericID := ""
		if item.ID > 0 {
			numericID = strconv.Itoa(item.ID)
		}

		companyName := strings.TrimSpace(item.CompanyName)
		if companyName == "" {
			companyName = strings.TrimSpace(item.CompanyShortName)
		}

		urlValue := strings.TrimSpace(item.URL)
		if urlValue == "" && urlCode != "" && numericID != "" {
			urlValue = fmt.Sprintf("https://www.investorgain.com/gmp/%s-ipo/%s/", urlCode, numericID)
		}

		if companyCode == "" {
			companyCode = urlCode
		}

		entries = append(entries, IPOUrlEntry{
			CompanyCode: companyCode,
			CompanyName: companyName,
			URL:         urlValue,
			URLCode:     urlCode,
			NumericID:   numericID,
		})
	}

	return normalizeIPOUrlEntries(entries), nil
}

func (c *InvestorGainAPIClient) GetIPOGMPPayload(ipoID string) (*IPOGMPPayload, error) {
	url := fmt.Sprintf("%s/ipo-gmp-read/%s/true", InvestorGainBaseURL, ipoID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result IPOGMPResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	points, err := decodeIPOGMPData(result.IpoGmpData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GMP data payload: %w", err)
	}

	if len(points) == 0 && strings.TrimSpace(result.IpoGmpTable) == "" {
		return nil, fmt.Errorf("%w", ErrNoGMPDataAvailable)
	}

	return &IPOGMPPayload{
		Key:         result.Key,
		CurrentTime: result.CurrentTime,
		TableHTML:   result.IpoGmpTable,
		DataPoints:  points,
	}, nil
}

func (c *InvestorGainAPIClient) GetIPOGMPData(ipoID string) ([]IPOGmpDataPoint, error) {
	payload, err := c.GetIPOGMPPayload(ipoID)
	if err != nil {
		return nil, err
	}

	if len(payload.DataPoints) == 0 {
		return nil, fmt.Errorf("%w", ErrNoGMPDataAvailable)
	}

	return payload.DataPoints, nil
}

func (c *InvestorGainAPIClient) GetIPOGMPDataWithRetry(ipoID string, maxRetries int) ([]IPOGmpDataPoint, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		data, err := c.GetIPOGMPData(ipoID)
		if err == nil {
			return data, nil
		}
		lastErr = err
		c.logger.WithError(err).Warnf("Retry %d/%d for IPO %s", i+1, maxRetries, ipoID)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func decodeIPOGMPData(raw json.RawMessage) ([]IPOGmpDataPoint, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	if trimmed[0] == '"' {
		var embedded string
		if err := json.Unmarshal(trimmed, &embedded); err != nil {
			return nil, err
		}
		embedded = strings.TrimSpace(embedded)
		if embedded == "" {
			return nil, nil
		}
		return decodeIPOGMPData(json.RawMessage(embedded))
	}

	if trimmed[0] == '[' {
		var points []IPOGmpDataPoint
		if err := json.Unmarshal(trimmed, &points); err != nil {
			return nil, err
		}
		return points, nil
	}

	if trimmed[0] == '{' {
		var point IPOGmpDataPoint
		if err := json.Unmarshal(trimmed, &point); err != nil {
			return nil, err
		}
		return []IPOGmpDataPoint{point}, nil
	}

	return nil, fmt.Errorf("unsupported ipoGmpData format")
}

var investorgainGMPURLPattern = regexp.MustCompile(`/gmp/([^/]+)-ipo/(\d+)/?`)

func normalizeIPOUrlEntries(entries []IPOUrlEntry) []IPOUrlEntry {
	normalized := make([]IPOUrlEntry, 0, len(entries))

	for _, entry := range entries {
		companyCode := normalizeCompanyCode(entry.CompanyCode)
		urlCode := normalizeCompanyCode(entry.URLCode)
		numericID := strings.TrimSpace(entry.NumericID)

		parsedCode, parsedNumericID := parseInvestorGainGMPURL(entry.URL)
		if urlCode == "" {
			urlCode = parsedCode
		}
		if numericID == "" {
			numericID = parsedNumericID
		}

		if companyCode == "" {
			companyCode = urlCode
		}
		if urlCode == "" {
			urlCode = companyCode
		}

		urlValue := strings.TrimSpace(entry.URL)
		if urlValue == "" && urlCode != "" && numericID != "" {
			urlValue = fmt.Sprintf("https://www.investorgain.com/gmp/%s-ipo/%s/", urlCode, numericID)
		}

		normalized = append(normalized, IPOUrlEntry{
			CompanyCode: companyCode,
			CompanyName: strings.TrimSpace(entry.CompanyName),
			URL:         urlValue,
			URLCode:     urlCode,
			NumericID:   numericID,
		})
	}

	return normalized
}

func parseInvestorGainGMPURL(url string) (string, string) {
	match := investorgainGMPURLPattern.FindStringSubmatch(strings.ToLower(strings.TrimSpace(url)))
	if len(match) < 3 {
		return "", ""
	}

	return normalizeCompanyCode(match[1]), strings.TrimSpace(match[2])
}

func normalizeCompanyCode(code string) string {
	normalized := strings.ToLower(strings.TrimSpace(code))
	suffixes := []string{"-ipo-details", "-details", "-ipo"}
	for _, suffix := range suffixes {
		normalized = strings.TrimSuffix(normalized, suffix)
	}
	return normalized
}
