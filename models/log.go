package models

import (
	"time"

	"github.com/google/uuid"
)

type IPOUpdateLog struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;default:gen_random_uuid()"`
	IPOID     uuid.UUID `json:"ipo_id"`
	FieldName string    `json:"field_name"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}
