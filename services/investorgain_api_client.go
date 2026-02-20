package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	InvestorGainBaseURL = "https://webnodejs.investorgain.com/cloud/new/ipo"
)

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
	Data []IPOUrlEntry `json:"data"`
}

type IPOUrlEntry struct {
	CompanyCode string `json:"company_code"`
	CompanyName string `json:"company_name"`
	URL         string `json:"url"`
}

type IPOGMPResponse struct {
	Status      string `json:"status"`
	IpoGmpTable string `json:"ipoGmpTable"`
	IpoGmpData  string `json:"ipoGmpData"`
}

type IPOGmpDataPoint struct {
	Date             string  `json:"date"`
	GMP              float64 `json:"gmp"`
	ListingDate      string  `json:"listing_date"`
	IPOPrice         float64 `json:"ipo_price"`
	EstimatedListing float64 `json:"estimated_listing"`
	EstimatedPercent float64 `json:"estimated_percent"`
	Sub2             float64 `json:"sub2"`
}

func (c *InvestorGainAPIClient) GetIPOUrlList() ([]IPOUrlEntry, error) {
	url := fmt.Sprintf("%s/ipo-url-lists", InvestorGainBaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

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

	return result.Data, nil
}

func (c *InvestorGainAPIClient) GetIPOGMPData(ipoID string) ([]IPOGmpDataPoint, error) {
	url := fmt.Sprintf("%s/ipo-gmp-read/%s/true", InvestorGainBaseURL, ipoID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

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

	if result.IpoGmpData == "" {
		return nil, fmt.Errorf("no GMP data available")
	}

	var dataPoints []IPOGmpDataPoint
	if err := json.Unmarshal([]byte(result.IpoGmpData), &dataPoints); err != nil {
		return nil, fmt.Errorf("failed to parse GMP data: %w", err)
	}

	return dataPoints, nil
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
