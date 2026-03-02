package jobs

import "testing"

// TestExtractRegistrarShortCode tests the extractRegistrarShortCode function
// with various input cases and variants
func TestExtractRegistrarShortCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Exact lowercase matches
		{
			name:     "exact match - kfin",
			input:    "kfin",
			expected: "KFIN",
		},
		{
			name:     "exact match - kfin technologies limited",
			input:    "kfin technologies limited",
			expected: "KFIN",
		},
		{
			name:     "exact match - kfin technologies pvt ltd",
			input:    "kfin technologies pvt ltd",
			expected: "KFIN",
		},

		// Mixed case inputs (case-insensitive matching)
		{
			name:     "mixed case - KFIN",
			input:    "KFIN",
			expected: "KFIN",
		},
		{
			name:     "mixed case - KFin",
			input:    "KFin",
			expected: "KFIN",
		},
		{
			name:     "mixed case - Kfin Technologies Limited",
			input:    "Kfin Technologies Limited",
			expected: "KFIN",
		},
		{
			name:     "mixed case - KFIN TECHNOLOGIES LIMITED",
			input:    "KFIN TECHNOLOGIES LIMITED",
			expected: "KFIN",
		},
		{
			name:     "mixed case - kFiN tEcHnOlOgIeS pVt LtD",
			input:    "kFiN tEcHnOlOgIeS pVt LtD",
			expected: "KFIN",
		},

		// KFin Technologies variant
		{
			name:     "kfin variant - KFin Technologies Pvt Ltd",
			input:    "KFin Technologies Pvt Ltd",
			expected: "KFIN",
		},

		// Bigshare Services
		{
			name:     "bigshare - Bigshare Services",
			input:    "Bigshare Services",
			expected: "BIGSHARE",
		},
		{
			name:     "bigshare - BIGSHARE SERVICES",
			input:    "BIGSHARE SERVICES",
			expected: "BIGSHARE",
		},
		{
			name:     "bigshare - Bigshare Services Pvt Ltd",
			input:    "Bigshare Services Pvt Ltd",
			expected: "BIGSHARE",
		},

		// MUFG Bank
		{
			name:     "mufg - MUFG",
			input:    "MUFG",
			expected: "MUFG",
		},
		{
			name:     "mufg - mufg",
			input:    "mufg",
			expected: "MUFG",
		},
		{
			name:     "mufg - MUFG Bank Japan Limited",
			input:    "MUFG Bank Japan Limited",
			expected: "MUFG",
		},
		{
			name:     "mufg - Mufg Bank Japan Limited",
			input:    "Mufg Bank Japan Limited",
			expected: "MUFG",
		},

		// Bank of India
		{
			name:     "bank of india - Bank of India",
			input:    "Bank of India",
			expected: "BOI",
		},
		{
			name:     "bank of india - BANK OF INDIA",
			input:    "BANK OF INDIA",
			expected: "BOI",
		},

		// Computershare
		{
			name:     "computershare - Computershare India Pvt Ltd",
			input:    "Computershare India Pvt Ltd",
			expected: "COMPUTERSHARE",
		},
		{
			name:     "computershare - COMPUTERSHARE INDIA PVT LTD",
			input:    "COMPUTERSHARE INDIA PVT LTD",
			expected: "COMPUTERSHARE",
		},

		// NSDL
		{
			name:     "nsdl - Nsdl Database Management Limited",
			input:    "Nsdl Database Management Limited",
			expected: "NSDL",
		},
		{
			name:     "nsdl - NSDL DATABASE MANAGEMENT LIMITED",
			input:    "NSDL DATABASE MANAGEMENT LIMITED",
			expected: "NSDL",
		},

		// CDSL
		{
			name:     "cdsl - Central Depository Services (India) Limited",
			input:    "Central Depository Services (India) Limited",
			expected: "CDSL",
		},
		{
			name:     "cdsl - CENTRAL DEPOSITORY SERVICES (INDIA) LIMITED",
			input:    "CENTRAL DEPOSITORY SERVICES (INDIA) LIMITED",
			expected: "CDSL",
		},

		// Unknown registrars
		{
			name:     "unknown - empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "unknown - unknown registrar",
			input:    "Unknown Registrar Corp",
			expected: "",
		},
		{
			name:     "unknown - random text",
			input:    "This is not a registrar",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRegistrarShortCode(tt.input)
			if result != tt.expected {
				t.Errorf("extractRegistrarShortCode(%q) = %q, want %q",
					tt.input, result, tt.expected)
			}
		})
	}
}
