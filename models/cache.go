package models

import (
	"time"

	"github.com/google/uuid"
)

type IPOResultCache struct {
	ID                uuid.UUID `json:"id" db:"id"`
	PanHash           string    `json:"pan_hash" db:"pan_hash"`
	IPOID             uuid.UUID `json:"ipo_id" db:"ipo_id"`
	Status            string    `json:"status" db:"status"`
	SharesAllotted    int       `json:"shares_allotted" db:"shares_allotted"`
	ApplicationNumber string    `json:"application_number" db:"application_number"`
	RefundStatus      string    `json:"refund_status" db:"refund_status"`
	Source            string    `json:"source" db:"source"`
	UserAgent         string    `json:"user_agent" db:"user_agent"`
	Timestamp         time.Time `json:"timestamp" db:"timestamp"`
	ExpiresAt         time.Time `json:"expires_at" db:"expires_at"`
	ConfidenceScore   int       `json:"confidence_score" db:"confidence_score"`
	DuplicateCount    int       `json:"duplicate_count" db:"duplicate_count"`
}
