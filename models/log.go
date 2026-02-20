package models

import (
	"time"

	"github.com/google/uuid"
)

type IPOUpdateLog struct {
	ID        uuid.UUID `json:"id" db:"id"`
	IPOID     uuid.UUID `json:"ipo_id" db:"ipo_id"`
	FieldName string    `json:"field_name" db:"field_name"`
	OldValue  string    `json:"old_value" db:"old_value"`
	NewValue  string    `json:"new_value" db:"new_value"`
	Source    string    `json:"source" db:"source"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
}
