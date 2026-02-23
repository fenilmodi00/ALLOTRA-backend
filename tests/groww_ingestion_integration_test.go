package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

var requiredGrowwSlugs = []string{
	"omnitech-engineering-ipo",
	"pngs-reva-ipo",
	"shree-ram-twistex-ipo",
}

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("Skipping integration test: TEST_DATABASE_URL or DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: cannot open DB: %v", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		t.Skipf("Skipping integration test: cannot ping DB: %v", err)
	}

	return db
}

func TestGrowwIngestion_RequiredSlugsExistInDB(t *testing.T) {
	db := openIntegrationDB(t)
	defer db.Close()

	for _, slug := range requiredGrowwSlugs {
		var stockID string
		var name string
		var foundSlug string
		var logoURL sql.NullString

		err := db.QueryRow(`
			SELECT stock_id, name, slug, logo_url
			FROM ipo_list
			WHERE slug = $1
			LIMIT 1
		`, slug).Scan(&stockID, &name, &foundSlug, &logoURL)

		if err != nil {
			if err == sql.ErrNoRows {
				t.Fatalf("required slug missing in DB: %s", slug)
			}
			t.Fatalf("failed querying slug %s: %v", slug, err)
		}

		if stockID == "" {
			t.Fatalf("stock_id is empty for slug %s", slug)
		}
		if name == "" {
			t.Fatalf("name is empty for slug %s", slug)
		}
		if foundSlug == "" {
			t.Fatalf("slug is empty in DB row for expected slug %s", slug)
		}
		if logoURL.Valid && logoURL.String == "" {
			t.Fatalf("logo_url is present but empty for slug %s", slug)
		}
	}
}

func TestGrowwIngestion_RequiredSlugsAppearInAPI(t *testing.T) {
	baseURL := os.Getenv("TEST_API_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/api/v1/ipos?limit=200")
	if err != nil {
		t.Skipf("Skipping API integration test: API not reachable at %s: %v", baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("Skipping API integration test: unexpected status %d from %s", resp.StatusCode, baseURL)
	}

	var payload struct {
		Success bool                     `json:"success"`
		Data    []map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode API response: %v", err)
	}

	if !payload.Success {
		t.Fatalf("API returned success=false")
	}

	found := make(map[string]bool, len(requiredGrowwSlugs))
	for _, item := range payload.Data {
		rawSlug, ok := item["slug"]
		if !ok || rawSlug == nil {
			continue
		}

		slug, ok := rawSlug.(string)
		if !ok {
			continue
		}
		found[slug] = true
	}

	for _, requiredSlug := range requiredGrowwSlugs {
		if !found[requiredSlug] {
			t.Fatalf("required slug missing in API response: %s", requiredSlug)
		}
	}

}
