package services

import (
	"encoding/json"
	"testing"
)

func TestDecodeIPOGMPDataSupportsArrayObjectAndEmbeddedString(t *testing.T) {
	t.Run("array payload", func(t *testing.T) {
		raw := json.RawMessage(`[{"gmp_date":"2026-01-15","gmp":"45"}]`)

		points, err := decodeIPOGMPData(raw)
		if err != nil {
			t.Fatalf("decodeIPOGMPData returned error: %v", err)
		}
		if len(points) != 1 {
			t.Fatalf("expected 1 point, got %d", len(points))
		}
		if got := points[0].Date.String(); got != "2026-01-15" {
			t.Fatalf("expected date 2026-01-15, got %q", got)
		}
	})

	t.Run("single object payload", func(t *testing.T) {
		raw := json.RawMessage(`{"gmp_date":"2026-01-16","gmp":"50"}`)

		points, err := decodeIPOGMPData(raw)
		if err != nil {
			t.Fatalf("decodeIPOGMPData returned error: %v", err)
		}
		if len(points) != 1 {
			t.Fatalf("expected 1 point, got %d", len(points))
		}
		if got := points[0].Date.String(); got != "2026-01-16" {
			t.Fatalf("expected date 2026-01-16, got %q", got)
		}
	})

	t.Run("embedded json string payload", func(t *testing.T) {
		arrayPayload := `[{"gmp_date":"2026-01-17","gmp":"55"}]`
		embedded, err := json.Marshal(arrayPayload)
		if err != nil {
			t.Fatalf("failed to marshal embedded payload: %v", err)
		}

		points, err := decodeIPOGMPData(embedded)
		if err != nil {
			t.Fatalf("decodeIPOGMPData returned error: %v", err)
		}
		if len(points) != 1 {
			t.Fatalf("expected 1 point, got %d", len(points))
		}
		if got := points[0].Date.String(); got != "2026-01-17" {
			t.Fatalf("expected date 2026-01-17, got %q", got)
		}
	})
}

func TestNormalizeIPOUrlEntriesExtractsURLCodeAndNumericID(t *testing.T) {
	entries := []IPOUrlEntry{
		{
			CompanyCode: "",
			CompanyName: "Acme Industries",
			URL:         "https://www.investorgain.com/gmp/acme-industries-ipo/12345/",
			URLCode:     "",
			NumericID:   "",
		},
	}

	normalized := normalizeIPOUrlEntries(entries)
	if len(normalized) != 1 {
		t.Fatalf("expected 1 normalized entry, got %d", len(normalized))
	}

	entry := normalized[0]
	if entry.CompanyCode != "acme-industries" {
		t.Fatalf("expected company code acme-industries, got %q", entry.CompanyCode)
	}
	if entry.URLCode != "acme-industries" {
		t.Fatalf("expected URL code acme-industries, got %q", entry.URLCode)
	}
	if entry.NumericID != "12345" {
		t.Fatalf("expected numeric ID 12345, got %q", entry.NumericID)
	}
}

func TestFlexibleStringUnmarshalSupportsStringNumberAndNull(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "string", input: `"  value  "`, expected: "value"},
		{name: "number", input: `123`, expected: "123"},
		{name: "null", input: `null`, expected: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var value FlexibleString
			if err := json.Unmarshal([]byte(tc.input), &value); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if got := value.String(); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
