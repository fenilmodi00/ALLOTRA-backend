package models

import (
	"time"

	"github.com/google/uuid"
)

type RegistrarCode struct {
	// Primary identification
	ID uuid.UUID `json:"id" db:"id"`

	// Foreign key reference
	IPOID uuid.UUID `json:"ipo_id" db:"ipo_id"`

	// Registrar information
	RegistrarShortCode   string  `json:"registrar_short_code" db:"registrar_short_code"`
	RegistrarCompanyCode *string `json:"registrar_company_code" db:"registrar_company_code"`

	// IPO reference
	IPOName *string `json:"ipo_name" db:"ipo_name"`

	// Matching and resolution
	MatchScore float64 `json:"match_score" db:"match_score"`
	IsResolved bool    `json:"is_resolved" db:"is_resolved"`

	// Audit fields
	LastAttemptedAt *time.Time `json:"last_attempted_at" db:"last_attempted_at"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
}
