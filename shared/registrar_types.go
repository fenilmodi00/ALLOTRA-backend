package shared

// DropdownOption represents an IPO option from registrar dropdown/selection lists
type DropdownOption struct {
	ID   string `json:"id"`   // Registrar-specific company code
	Name string `json:"name"` // IPO company name
}

// AllotmentResult contains detailed allotment data from registrar checks
type AllotmentResult struct {
	Status         string // Status constants: ALLOTTED, NOT_ALLOTTED, NOT_FOUND, ERROR
	ApplicationNo  string // Application number if available
	Name           string // Applicant name
	SharesApplied  int    // Number of shares applied for
	SharesAllotted int    // Number of shares allotted (0 if not allotted)
	Category       string // Application category (e.g., "Retail Individual Investor")
}

// Allotment status constants
const (
	StatusAllotted    = "ALLOTTED"
	StatusNotAllotted = "NOT_ALLOTTED"
	StatusNotFound    = "NOT_FOUND"
	StatusError       = "ERROR"
)
