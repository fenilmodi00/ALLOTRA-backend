package mufg

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/sirupsen/logrus"
)

const (
	mufgBaseURL = "https://in.mpms.mufg.com"
	mufgPage    = "/Initial_Offer/public-issues.html"
	mufgAPI     = "/Initial_Offer/IPO.aspx"
	mufgTimeout = 15 * time.Second
)

type MUFGAPIResponse struct {
	XMLName xml.Name        `xml:"NewDataSet"`
	Table   []MUFGTableRow  `xml:"Table"`
	Table1  []MUFGTable1Row `xml:"Table1"`
}

type MUFGTableRow struct {
	XMLName     xml.Name `xml:"Table"`
	ALLOT       string   `xml:"ALLOT"`
	APPLNO      string   `xml:"APPLNO"`
	NAME        string   `xml:"NAME"`
	PAN         string   `xml:"PAN"`
	DEPO        string   `xml:"DEPO"`
	CLIENTID    string   `xml:"CLIENTID"`
	IFSC        string   `xml:"IFSC"`
	BOID        string   `xml:"BOID"`
	UQID        string   `xml:"UQID"`
	APPAMT      string   `xml:"APPAMT"`
	ALLOTMRP    string   `xml:"ALLOTMRP"`
	ALLOTSHARES string   `xml:"ALLOTSHARES"`
	REFUNDAMT   string   `xml:"REFUNDAMT"`
	MAKEPV      string   `xml:"MAKEPV"`
	MATCH       string   `xml:"match"`
	PULL        string   `xml:"pull"`
}

type MUFGTable1Row struct {
	XMLName xml.Name `xml:"Table1"`
	Msg     string   `xml:"Msg"`
}

type MUFGGenerateTokenResponse struct {
	D string `json:"d"`
}

type Client struct {
	logger *logrus.Entry
}

func NewClient() *Client {
	return &Client{
		logger: logrus.WithField("component", "mufg_checker"),
	}
}

func (mc *Client) newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: mufgTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar: jar,
	}
}

func (mc *Client) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://in.mpms.mufg.com")
	req.Header.Set("Referer", "https://in.mpms.mufg.com/Initial_Offer/public-issues.html")
}

func (mc *Client) generateToken(ctx context.Context, client *http.Client) (string, error) {
	apiURL := mufgBaseURL + mufgAPI + "/generateToken"

	payload := map[string]string{}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	mc.setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token API returned HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response body: %w", err)
	}

	var tokenResp MUFGGenerateTokenResponse
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.D, nil
}

func (mc *Client) CheckAllotment(ctx context.Context, companyCode string, pan string) (*shared.AllotmentResult, error) {
	client := mc.newHTTPClient()

	if companyCode == "" {
		return nil, fmt.Errorf("company code is required for MUFG allotment check")
	}

	token, err := mc.generateToken(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	mc.logger.WithFields(logrus.Fields{
		"company_id": companyCode,
		"pan":        maskPAN(pan),
	}).Debug("Making API request")

	apiURL := mufgBaseURL + mufgAPI + "/SearchOnPan"

	// CHKVAL: 1 = PAN, 2 = Application No, 3 = DPID, 4 = IFSC
	payload := map[string]string{
		"clientid": companyCode,
		"PAN":      pan,
		"IFSC":     "",
		"CHKVAL":   "1", // PAN search
		"token":    token,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %w", err)
	}
	mc.setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return mc.parseResponse(bodyBytes)
}

func (mc *Client) parseResponse(body []byte) (*shared.AllotmentResult, error) {
	// The response is JSON with the XML in the "d" field
	var jsonResp struct {
		D string `json:"d"`
	}
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Now parse the XML from the "d" field
	var resp MUFGAPIResponse
	if err := xml.Unmarshal([]byte(jsonResp.D), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	if len(resp.Table1) > 0 && resp.Table1[0].Msg != "" {
		result := &shared.AllotmentResult{
			Status: shared.StatusNotFound,
		}
		return result, nil
	}

	if len(resp.Table) == 0 {
		return &shared.AllotmentResult{
			Status: shared.StatusNotFound,
		}, nil
	}

	row := resp.Table[0]
	allotment := parseIntSafe(row.ALLOT)

	result := &shared.AllotmentResult{
		ApplicationNo:  row.APPLNO,
		SharesApplied:  parseIntSafe(row.ALLOTMRP),
		SharesAllotted: allotment,
		Name:           row.NAME,
	}

	if allotment > 0 {
		result.Status = shared.StatusAllotted
	} else {
		result.Status = shared.StatusNotAllotted
	}

	return result, nil
}

func (mc *Client) findCompanyID(ctx context.Context, client *http.Client, companyName string) (string, error) {
	apiURL := mufgBaseURL + mufgAPI + "/GetDetails"

	payload := map[string]string{}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	mc.setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var respObj struct {
		D string `json:"d"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respObj); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(respObj.D))
	if err != nil {
		return "", err
	}

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

	doc.Find("option").Each(func(i int, s *goquery.Selection) {
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
			bestMatch = match{value: val, text: text, score: score}
		}
	})

	if bestMatch.value == "" {
		return "", fmt.Errorf("no matching company found for '%s'", companyName)
	}

	return bestMatch.value, nil
}

func (mc *Client) GetActiveIPOs(ctx context.Context) ([]shared.DropdownOption, error) {
	client := mc.newHTTPClient()

	apiURL := mufgBaseURL + mufgAPI + "/GetDetails"

	payload := map[string]string{}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	mc.setHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET request failed: %w", err)
	}
	defer resp.Body.Close()

	var respObj struct {
		D string `json:"d"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respObj); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(respObj.D))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var options []shared.DropdownOption
	doc.Find("option").Each(func(i int, s *goquery.Selection) {
		val, exists := s.Attr("value")
		text := strings.TrimSpace(s.Text())
		if exists && val != "0" && val != "" && !strings.Contains(strings.ToLower(text), "select company") {
			options = append(options, shared.DropdownOption{ID: val, Name: text})
		}
	})

	return options, nil
}

func (mc *Client) MatchCompanyName(companyName string, options []shared.DropdownOption) (string, float64) {
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
