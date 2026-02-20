package models

const (
	StatusUpcoming  = "UPCOMING"
	StatusLive      = "LIVE"
	StatusClosed    = "CLOSED"
	StatusResultOut = "RESULT_OUT"
	StatusListed    = "LISTED"
	StatusUnknown   = "Unknown"
)

func ValidStatuses() []string {
	return []string{
		StatusUpcoming,
		StatusLive,
		StatusClosed,
		StatusResultOut,
		StatusListed,
		StatusUnknown,
	}
}

func IsValidStatus(status string) bool {
	for _, s := range ValidStatuses() {
		if s == status {
			return true
		}
	}
	return false
}

const (
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
	JobStatusPartial   = "partial"
)
