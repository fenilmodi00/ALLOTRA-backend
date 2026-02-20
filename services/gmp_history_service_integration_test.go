package services

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// getTestDB returns a test database connection or skips the test
func getTestDB(t *testing.T) *sql.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@localhost:5432/ipo_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping integration test: failed to connect to database: %v", err)
		return nil
	}

	// Test connection
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
		return nil
	}

	return db
}

// cleanupTestDataByIPO removes test data from the database for a specific IPO
func cleanupTestDataByIPO(t *testing.T, db *sql.DB, ipoID string) {
	_, err := db.Exec("DELETE FROM gmp_price_history WHERE ipo_id = $1", ipoID)
	if err != nil {
		t.Logf("Warning: failed to cleanup test data: %v", err)
	}
}

// TestSavePriceHistory_Integration tests saving price history to database
func TestSavePriceHistory_Integration(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	service := NewGMPHistoryService(db)

	// Create a test IPO first
	testIPOID := uuid.New().String()
	testCompanyCode := "TESTIPO"

	// Insert test IPO
	_, err := db.Exec(`
		INSERT INTO ipo_list (id, stock_id, name, company_code, registrar, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, testIPOID, "TEST001", "Test IPO Ltd", testCompanyCode, "Test Registrar", "LIVE")
	if err != nil {
		t.Fatalf("Failed to create test IPO: %v", err)
	}
	defer func() {
		db.Exec("DELETE FROM ipo_list WHERE id = $1", testIPOID)
	}()
	defer cleanupTestDataByIPO(t, db, testIPOID)

	// Create test price history collection
	now := time.Now()
	collection := &models.GMPPriceHistoryCollection{
		IPOID:       testIPOID,
		IPOName:     "Test IPO Ltd",
		CompanyCode: testCompanyCode,
		Entries: []models.GMPPriceHistoryEntry{
			{
				RecordDate:         now.AddDate(0, 0, -2),
				IPOPrice:           100.0,
				GMPValue:           10.0,
				EstimatedListing:   110.0,
				ListingPercent:     10.0,
				EstimatedProfit:    10.0,
				SubscriptionStatus: "10x subscribed",
				Sub2Sauda:          5.0,
				LastUpdated:        "2 days ago",
				DataSource:         "investorgain.com",
			},
			{
				RecordDate:         now.AddDate(0, 0, -1),
				IPOPrice:           100.0,
				GMPValue:           15.0,
				EstimatedListing:   115.0,
				ListingPercent:     15.0,
				EstimatedProfit:    15.0,
				SubscriptionStatus: "15x subscribed",
				Sub2Sauda:          7.5,
				LastUpdated:        "1 day ago",
				DataSource:         "investorgain.com",
			},
		},
	}

	// Test saving
	err = service.SavePriceHistory(collection)
	if err != nil {
		t.Fatalf("Failed to save price history: %v", err)
	}

	// Verify data was saved
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM gmp_price_history WHERE ipo_id = $1", testIPOID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count saved entries: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 entries to be saved, got %d", count)
	}

	// Test upsert behavior - update existing entry
	collection.Entries[0].GMPValue = 20.0
	collection.Entries[0].EstimatedListing = 120.0
	collection.Entries[0].ListingPercent = 20.0

	err = service.SavePriceHistory(collection)
	if err != nil {
		t.Fatalf("Failed to upsert price history: %v", err)
	}

	// Verify count is still 2 (no duplicates)
	err = db.QueryRow("SELECT COUNT(*) FROM gmp_price_history WHERE ipo_id = $1", testIPOID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count entries after upsert: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 entries after upsert (no duplicates), got %d", count)
	}

	// Verify the value was updated
	var gmpValue float64
	err = db.QueryRow(`
		SELECT gmp_value FROM gmp_price_history 
		WHERE ipo_id = $1 AND record_date = $2
	`, testIPOID, collection.Entries[0].RecordDate).Scan(&gmpValue)
	if err != nil {
		t.Fatalf("Failed to query updated entry: %v", err)
	}

	if gmpValue != 20.0 {
		t.Errorf("Expected GMP value to be updated to 20.0, got %.2f", gmpValue)
	}
}

// TestGetPriceHistoryByIPO_Integration tests retrieving price history from database
func TestGetPriceHistoryByIPO_Integration(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	service := NewGMPHistoryService(db)

	// Create a test IPO
	testIPOID := uuid.New().String()
	testCompanyCode := "TESTIPO2"

	_, err := db.Exec(`
		INSERT INTO ipo_list (id, stock_id, name, company_code, registrar, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, testIPOID, "TEST002", "Test IPO 2 Ltd", testCompanyCode, "Test Registrar", "LIVE")
	if err != nil {
		t.Fatalf("Failed to create test IPO: %v", err)
	}
	defer func() {
		db.Exec("DELETE FROM ipo_list WHERE id = $1", testIPOID)
	}()
	defer cleanupTestDataByIPO(t, db, testIPOID)

	// Insert test price history data
	now := time.Now()
	dates := []time.Time{
		now.AddDate(0, 0, -5),
		now.AddDate(0, 0, -3),
		now.AddDate(0, 0, -1),
	}

	for i, date := range dates {
		_, err := db.Exec(`
			INSERT INTO gmp_price_history (
				id, ipo_id, company_code, record_date, ipo_price, gmp_value,
				estimated_listing, listing_percent, estimated_profit,
				subscription_status, sub2_sauda, last_updated, data_source
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, uuid.New().String(), testIPOID, testCompanyCode, date, 100.0, float64(10+i*5),
			float64(110+i*5), float64(10+i*5), float64(10+i*5),
			"subscribed", 5.0, "test", "investorgain.com")
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Test 1: Get all history without date range
	collection, err := service.GetPriceHistoryByIPO(testIPOID, nil)
	if err != nil {
		t.Fatalf("Failed to get price history: %v", err)
	}

	if collection.TotalRecords != 3 {
		t.Errorf("Expected 3 entries, got %d", collection.TotalRecords)
	}

	if len(collection.Entries) != 3 {
		t.Errorf("Expected 3 entries in collection, got %d", len(collection.Entries))
	}

	// Verify entries are ordered by date descending
	if len(collection.Entries) >= 2 {
		if collection.Entries[0].RecordDate.Before(collection.Entries[1].RecordDate) {
			t.Error("Expected entries to be ordered by date descending")
		}
	}

	// Test 2: Get history with date range filter
	dateRange := &models.DateRange{
		StartDate: now.AddDate(0, 0, -4),
		EndDate:   now.AddDate(0, 0, -2),
	}

	filteredCollection, err := service.GetPriceHistoryByIPO(testIPOID, dateRange)
	if err != nil {
		t.Fatalf("Failed to get filtered price history: %v", err)
	}

	if filteredCollection.TotalRecords != 1 {
		t.Errorf("Expected 1 entry in date range, got %d", filteredCollection.TotalRecords)
	}

	// Test 3: Get history for non-existent IPO
	_, err = service.GetPriceHistoryByIPO(uuid.New().String(), nil)
	if err == nil {
		t.Error("Expected error for non-existent IPO, got nil")
	}
}
