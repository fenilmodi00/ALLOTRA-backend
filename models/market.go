package models

// MarketIndex represents a stock market index with current value and change information
type MarketIndex struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Value         float64 `json:"value"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	IsPositive    bool    `json:"is_positive"`
}
