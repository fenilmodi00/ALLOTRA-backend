package services

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestFuzzyMatchRejectsSubstringCollisions verifies that the fixed fuzzy matching
// logic does NOT match short company codes as substrings inside longer unrelated
// URL slugs. This was the root cause of the numeric_id=612 collision bug where
// KRL, MIIL, and AT&SL all resolved to the same wrong InvestorGain ID.
func TestFuzzyMatchRejectsSubstringCollisions(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{
		logger:    logrus.New(),
		apiClient: NewInvestorGainAPIClient(),
	}

	// Simulate the HTML content from InvestorGain listing page.
	// The key: "krl" should NOT match "sparkrl" or "krl-technologies" belonging to a different IPO.
	htmlContent := `
		<a href="/gmp/sparkrl-ipo/612/">Spark RL IPO</a>
		<a href="/gmp/acme-krl-systems-ipo/700/">Acme KRL Systems IPO</a>
		<a href="/gmp/mobilise-app-lab-ipo/1869/">Mobilise App Lab IPO</a>
	`

	t.Run("short code krl must not substring-match sparkrl", func(t *testing.T) {
		numericID, found := scraper.findByFuzzyCompanyCode(htmlContent, "krl")
		if found {
			t.Errorf("findByFuzzyCompanyCode matched 'krl' to numeric_id=%s — expected no match (substring collision)", numericID)
		}
	})

	t.Run("short code miil must not substring-match longer codes", func(t *testing.T) {
		htmlWithMiil := `
			<a href="/gmp/premiil-tech-ipo/612/">Premiil Tech IPO</a>
			<a href="/gmp/real-target-ipo/999/">Real Target IPO</a>
		`
		numericID, found := scraper.findByFuzzyCompanyCode(htmlWithMiil, "miil")
		if found {
			t.Errorf("findByFuzzyCompanyCode matched 'miil' to numeric_id=%s — expected no match", numericID)
		}
	})

	t.Run("exact variation match still works", func(t *testing.T) {
		// generateCompanyCodeVariations("mobilise-app-lab") includes "mobilise-app-lab" itself
		numericID, found := scraper.findByFuzzyCompanyCode(htmlContent, "mobilise-app-lab")
		if !found {
			t.Error("findByFuzzyCompanyCode should match exact variation 'mobilise-app-lab'")
		} else if numericID != "1869" {
			t.Errorf("expected numeric_id=1869, got %s", numericID)
		}
	})
}

// TestGenerateCompanyCodeVariationsProducesExactMatches verifies that the variation
// generator produces useful variations but does NOT create overly broad patterns.
func TestGenerateCompanyCodeVariationsProducesExactMatches(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{logger: logrus.New()}

	t.Run("short code produces expected variations", func(t *testing.T) {
		variations := scraper.generateCompanyCodeVariations("krl")
		found := false
		for _, v := range variations {
			if v == "krl" {
				found = true
				break
			}
		}
		if !found {
			t.Error("variations should include the original code 'krl'")
		}
	})

	t.Run("code with hyphens produces separator variations", func(t *testing.T) {
		variations := scraper.generateCompanyCodeVariations("mobilise-app-lab")
		hasNoHyphen := false
		for _, v := range variations {
			if v == "mobiliseapplab" {
				hasNoHyphen = true
				break
			}
		}
		if !hasNoHyphen {
			t.Error("variations should include hyphen-removed version 'mobiliseapplab'")
		}
	})

	t.Run("does not strip semantically meaningful suffixes like -india", func(t *testing.T) {
		// "manilam-industries-india" must NOT produce "manilam-industries"
		// because that's a DIFFERENT company's IPO on InvestorGain.
		variations := scraper.generateCompanyCodeVariations("manilam-industries-india")
		for _, v := range variations {
			if v == "manilam-industries" {
				t.Error("variations must NOT strip '-india' suffix — 'manilam-industries' is a different IPO")
			}
		}
	})

	t.Run("does not strip -ltd or -technologies suffixes", func(t *testing.T) {
		variations := scraper.generateCompanyCodeVariations("acme-technologies")
		for _, v := range variations {
			if v == "acme" {
				t.Error("variations must NOT strip '-technologies' — it distinguishes companies")
			}
		}
	})

	t.Run("strips only noise suffixes like -ipo and -details", func(t *testing.T) {
		variations := scraper.generateCompanyCodeVariations("krl-ipo")
		hasStripped := false
		for _, v := range variations {
			if v == "krl" {
				hasStripped = true
				break
			}
		}
		if !hasStripped {
			t.Error("variations should strip '-ipo' suffix to produce 'krl'")
		}
	})
}

// TestFindNumericIDFromAPIURLListRejectsShortCodeCollisions is a unit test
// that exercises findNumericIDFromAPIURLList with a mock candidate list,
// verifying that short codes don't substring-match unrelated entries.
// We can't call findNumericIDFromAPIURLList directly without mocking the API,
// but we can test the matching logic indirectly through findByFuzzyCompanyCode
// and findByExactCompanyCode which use the same patterns.
func TestExactCompanyCodeMatchDoesNotSubstringMatch(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{logger: logrus.New()}

	htmlContent := `
		<a href="/gmp/sparkrl-ipo/612/">Spark RL IPO</a>
		<a href="/gmp/krl-ipo/2417/">Kiaasa Retail IPO</a>
	`

	t.Run("exact match finds correct ID when code exists in URL", func(t *testing.T) {
		numericID, found := scraper.findByExactCompanyCode(htmlContent, "krl")
		if !found {
			t.Error("findByExactCompanyCode should find 'krl' in href='/gmp/krl-ipo/2417/'")
		} else if numericID != "2417" {
			t.Errorf("expected numeric_id=2417, got %s", numericID)
		}
	})

	t.Run("exact match does not match substring sparkrl for code krl", func(t *testing.T) {
		// Remove the exact krl entry, leaving only sparkrl
		htmlNoExact := `<a href="/gmp/sparkrl-ipo/612/">Spark RL IPO</a>`
		numericID, found := scraper.findByExactCompanyCode(htmlNoExact, "krl")
		if found {
			t.Errorf("findByExactCompanyCode should NOT match 'krl' inside 'sparkrl', but got numeric_id=%s", numericID)
		}
	})
}

// TestNameSimilarityDoesNotMatchUnrelatedCompanies tests that name similarity
// matching rejects clearly unrelated company names.
func TestNameSimilarityDoesNotMatchUnrelatedCompanies(t *testing.T) {
	scraper := &GMPPriceHistoryScraper{logger: logrus.New()}

	tests := []struct {
		name1    string
		name2    string
		expectLo float64 // should be below this
	}{
		{"Kiaasa Retail Ltd", "Spark RL Technologies", 0.6},
		{"Manilam Industries India Ltd", "Premier Mills IPO", 0.6},
		{"Accord Transformer & Switchgear Ltd", "Atlas Solutions IPO", 0.6},
	}

	for _, tc := range tests {
		t.Run(tc.name1+" vs "+tc.name2, func(t *testing.T) {
			n1 := scraper.normalizeIPOName(tc.name1)
			n2 := scraper.normalizeIPOName(tc.name2)
			words := splitWords(n1)
			score := scraper.calculateNameSimilarity(n1, n2, words)
			if score >= tc.expectLo {
				t.Errorf("name similarity between %q and %q = %.2f, expected < %.2f", tc.name1, tc.name2, score, tc.expectLo)
			}
		})
	}
}

func splitWords(s string) []string {
	return strings.Fields(s)
}
