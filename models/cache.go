package models

import (
	"time"

	"github.com/google/uuid"
)

type IPOResultCache struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid()"`
	PanHash           string    `json:"pan_hash"`
	IPOID             uuid.UUID `json:"ipo_id"`
	Status            string    `json:"status"`
	SharesAllotted    int       `json:"shares_allotted"`
	ApplicationNumber string    `json:"application_number"`
	RefundStatus      string    `json:"refund_status"`
	Source            string    `json:"source"`
	UserAgent         string    `json:"user_agent"`
	Timestamp         time.Time `json:"timestamp"`
	ExpiresAt         time.Time `json:"expires_at"`
	ConfidenceScore   int       `json:"confidence_score"`
	DuplicateCount    int       `json:"duplicate_count"`
}
