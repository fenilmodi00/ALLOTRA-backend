package services

import "fmt"

// GMPHistoryFailureType categorizes why GMP scraping failed
type GMPHistoryFailureType string

const (
	FailureTypeNOIGID     GMPHistoryFailureType = "NO_IG_ID"      // Could not find InvestorGain ID
	FailureTypeNOGMPData  GMPHistoryFailureType = "NO_GMP_DATA"   // Found ID but no GMP data yet
	FailureTypeParseError GMPHistoryFailureType = "PARSE_ERROR"   // HTML parsing failed
	FailureTypeNetworkErr GMPHistoryFailureType = "NETWORK_ERROR" // HTTP request failed
)

// GMPHistoryError wraps errors with failure type for clearer logging
type GMPHistoryError struct {
	FailureType GMPHistoryFailureType
	Message     string
	Err         error
}

func (e *GMPHistoryError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.FailureType, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.FailureType, e.Message)
}

func (e *GMPHistoryError) Unwrap() error {
	return e.Err
}

// NewGMPHistoryError creates a typed error
func NewGMPHistoryError(failureType GMPHistoryFailureType, message string, err error) *GMPHistoryError {
	return &GMPHistoryError{
		FailureType: failureType,
		Message:     message,
		Err:         err,
	}
}
