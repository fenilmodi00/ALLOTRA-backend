package services

import (
	"database/sql"
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// TestArchiveOldHistory_AgeBasedArchival tests age-based archival logic
func TestArchiveOldHistory_AgeBasedArchival(t *testing.T) {
	// Skip if no database connection available
	db := setupTestDB(t)
	if db == nil {
		t.Skip("Database not available for testing")
		return
	}
	defer db.Close()

	service := NewGMPHistoryService(db)
	defer service.Close()

	// Clean up test data
	defer cleanupTestData(t, db)

	// Create test IPO
	ipoID := createTestIPO(t, db, "TEST-ARCHIVE-AGE")

	// Insert test data with various ages
	now := time.Now()
	testData := []struct {
		recordDate    time.Time
		shouldArchive bool
	}{
		{now.AddDate(-3, 0, 0), true},  // 3 years old - should archive
		{now.AddDate(-2, -6, 0), true}, // 2.5 years old - should archive
		{now.AddDate(-1, 0, 0), false}, // 1 year old - should keep
		{now.AddDate(0, -6, 0), false}, // 6 months old - should keep
		{now, false},                   // Today - should keep
	}

	for i, td := range testData {
		entry := &models.GMPPriceHistoryEntry{
			ID:               uuid.New().String(),
			IPOID:            ipoID,
			CompanyCode:      "TEST-ARCHIVE-AGE",
			RecordDate:       td.recordDate,
			IPOPrice:         100.0,
			GMPValue:         float64(i * 10),
			EstimatedListing: 100.0 + float64(i*10),
			ListingPercent:   float64(i * 10),
			DataSource:       "test",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		err := insertTestHistoryEntry(t, db, entry)
		if err != nil {
			t.Fatalf("Failed to insert test entry: %v", err)
		}
	}

	// Archive entries older than 2 years
	cutoffDate := now.AddDate(-2, 0, 0)
	report, err := service.ArchiveOldHistory(cutoffDate)
	if err != nil {
		t.Fatalf("ArchiveOldHistory failed: %v", err)
	}

	// Verify report
	if report.ArchivalStatus != "success" {
		t.Errorf("Expected archival status 'success', got '%s'", report.ArchivalStatus)
	}

	if report.RecordsArchived != 2 {
		t.Errorf("Expected 2 records archived, got %d", report.RecordsArchived)
	}

	if report.TriggerType != "age-based" {
		t.Errorf("Expected trigger type 'age-based', got '%s'", report.TriggerType)
	}

	// Verify active records count
	var activeCount int
	err = db.QueryRow("SELECT COUNT(*) FROM gmp_price_history WHERE ipo_id = $1", ipoID).Scan(&activeCount)
	if err != nil {
		t.Fatalf("Failed to count active records: %v", err)
	}

	expectedActive := 3 // 3 records should remain
	if activeCount != expectedActive {
		t.Errorf("Expected %d active records, got %d", expectedActive, activeCount)
	}

	// Verify archived records count
	var archivedCount int
	err = db.QueryRow("SELECT COUNT(*) FROM gmp_price_history_archive WHERE ipo_id = $1", ipoID).Scan(&archivedCount)
	if err != nil {
		t.Fatalf("Failed to count archived records: %v", err)
	}

	expectedArchived := 2 // 2 records should be archived
	if archivedCount != expectedArchived {
		t.Errorf("Expected %d archived records, got %d", expectedArchived, archivedCount)
	}
}

// TestArchiveByVolume_VolumeBasedArchival tests volume-based archival logic
func TestArchiveByVolume_VolumeBasedArchival(t *testing.T) {
	// Skip if no database connection available
	db := setupTestDB(t)
	if db == nil {
		t.Skip("Database not available for testing")
		return
	}
	defer db.Close()

	service := NewGMPHistoryService(db)
	defer service.Close()

	// Clean up test data
	defer cleanupTestData(t, db)

	// Create test IPO
	ipoID := createTestIPO(t, db, "TEST-ARCHIVE-VOL")

	// Insert 100 test records
	now := time.Now()
	for i := 0; i < 100; i++ {
		entry := &models.GMPPriceHistoryEntry{
			ID:               uuid.New().String(),
			IPOID:            ipoID,
			CompanyCode:      "TEST-ARCHIVE-VOL",
			RecordDate:       now.AddDate(0, 0, -i), // Each day going back
			IPOPrice:         100.0,
			GMPValue:         float64(i),
			EstimatedListing: 100.0 + float64(i),
			ListingPercent:   float64(i),
			DataSource:       "test",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		err := insertTestHistoryEntry(t, db, entry)
		if err != nil {
			t.Fatalf("Failed to insert test entry %d: %v", i, err)
		}
	}

	// Set volume threshold to 50 (should trigger archival)
	volumeThreshold := 50
	report, err := service.ArchiveByVolume(volumeThreshold)
	if err != nil {
		t.Fatalf("ArchiveByVolume failed: %v", err)
	}

	// Verify report
	if report.ArchivalStatus != "success" {
		t.Errorf("Expected archival status 'success', got '%s'", report.ArchivalStatus)
	}

	if report.TriggerType != "volume-based" {
		t.Errorf("Expected trigger type 'volume-based', got '%s'", report.TriggerType)
	}

	// Should archive approximately 20% of records (20 records)
	expectedArchived := 20
	tolerance := 5 // Allow some tolerance
	if report.RecordsArchived < expectedArchived-tolerance || report.RecordsArchived > expectedArchived+tolerance {
		t.Errorf("Expected approximately %d records archived, got %d", expectedArchived, report.RecordsArchived)
	}

	// Verify active records count is below threshold
	var activeCount int
	err = db.QueryRow("SELECT COUNT(*) FROM gmp_price_history WHERE ipo_id = $1", ipoID).Scan(&activeCount)
	if err != nil {
		t.Fatalf("Failed to count active records: %v", err)
	}

	if activeCount >= 100 {
		t.Errorf("Expected active records to be reduced, got %d", activeCount)
	}
}

// TestArchiveByVolume_BelowThreshold tests that no archival occurs when below threshold
func TestArchiveByVolume_BelowThreshold(t *testing.T) {
	// Skip if no database connection available
	db := setupTestDB(t)
	if db == nil {
		t.Skip("Database not available for testing")
		return
	}
	defer db.Close()

	service := NewGMPHistoryService(db)
	defer service.Close()

	// Clean up test data
	defer cleanupTestData(t, db)

	// Create test IPO
	ipoID := createTestIPO(t, db, "TEST-ARCHIVE-BELOW")

	// Insert only 10 test records
	now := time.Now()
	for i := 0; i < 10; i++ {
		entry := &models.GMPPriceHistoryEntry{
			ID:               uuid.New().String(),
			IPOID:            ipoID,
			CompanyCode:      "TEST-ARCHIVE-BELOW",
			RecordDate:       now.AddDate(0, 0, -i),
			IPOPrice:         100.0,
			GMPValue:         float64(i),
			EstimatedListing: 100.0 + float64(i),
			ListingPercent:   float64(i),
			DataSource:       "test",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		err := insertTestHistoryEntry(t, db, entry)
		if err != nil {
			t.Fatalf("Failed to insert test entry %d: %v", i, err)
		}
	}

	// Set volume threshold to 100 (should NOT trigger archival)
	volumeThreshold := 100
	report, err := service.ArchiveByVolume(volumeThreshold)
	if err != nil {
		t.Fatalf("ArchiveByVolume failed: %v", err)
	}

	// Verify no archival occurred
	if report.RecordsArchived != 0 {
		t.Errorf("Expected 0 records archived, got %d", report.RecordsArchived)
	}

	if report.ArchivalStatus != "success" {
		t.Errorf("Expected archival status 'success', got '%s'", report.ArchivalStatus)
	}
}

// TestCheckArchivalNeeded tests the archival check logic
func TestCheckArchivalNeeded(t *testing.T) {
	// Skip if no database connection available
	db := setupTestDB(t)
	if db == nil {
		t.Skip("Database not available for testing")
		return
	}
	defer db.Close()

	service := NewGMPHistoryService(db)
	defer service.Close()

	// Clean up test data
	defer cleanupTestData(t, db)

	// Create test IPO
	ipoID := createTestIPO(t, db, "TEST-CHECK-ARCHIVAL")

	// Insert old records
	now := time.Now()
	for i := 0; i < 5; i++ {
		entry := &models.GMPPriceHistoryEntry{
			ID:               uuid.New().String(),
			IPOID:            ipoID,
			CompanyCode:      "TEST-CHECK-ARCHIVAL",
			RecordDate:       now.AddDate(-3, 0, -i), // 3 years old
			IPOPrice:         100.0,
			GMPValue:         float64(i),
			EstimatedListing: 100.0 + float64(i),
			ListingPercent:   float64(i),
			DataSource:       "test",
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		err := insertTestHistoryEntry(t, db, entry)
		if err != nil {
			t.Fatalf("Failed to insert test entry %d: %v", i, err)
		}
	}

	// Check if archival is needed (2 year threshold)
	needed, reason, err := service.CheckArchivalNeeded(1000, 2)
	if err != nil {
		t.Fatalf("CheckArchivalNeeded failed: %v", err)
	}

	if !needed {
		t.Errorf("Expected archival to be needed, but it wasn't. Reason: %s", reason)
	}

	if reason == "" {
		t.Errorf("Expected a reason for archival, got empty string")
	}

	t.Logf("Archival needed: %v, Reason: %s", needed, reason)
}

// TestGetArchivalStatistics tests the archival statistics retrieval
func TestGetArchivalStatistics(t *testing.T) {
	// Skip if no database connection available
	db := setupTestDB(t)
	if db == nil {
		t.Skip("Database not available for testing")
		return
	}
	defer db.Close()

	service := NewGMPHistoryService(db)
	defer service.Close()

	// Clean up test data
	defer cleanupTestData(t, db)

	// Get statistics
	stats, err := service.GetArchivalStatistics()
	if err != nil {
		t.Fatalf("GetArchivalStatistics failed: %v", err)
	}

	// Verify statistics structure
	if _, ok := stats["active_records"]; !ok {
		t.Errorf("Expected 'active_records' in statistics")
	}

	if _, ok := stats["archived_records"]; !ok {
		t.Errorf("Expected 'archived_records' in statistics")
	}

	t.Logf("Archival statistics: %+v", stats)
}

// TestGetArchivalHistory tests the archival history retrieval
func TestGetArchivalHistory(t *testing.T) {
	// Skip if no database connection available
	db := setupTestDB(t)
	if db == nil {
		t.Skip("Database not available for testing")
		return
	}
	defer db.Close()

	service := NewGMPHistoryService(db)
	defer service.Close()

	// Clean up test data
	defer cleanupTestData(t, db)

	// Get archival history
	history, err := service.GetArchivalHistory(10)
	if err != nil {
		t.Fatalf("GetArchivalHistory failed: %v", err)
	}

	// History might be empty if no archival operations have been performed
	t.Logf("Found %d archival operations in history", len(history))

	for i, report := range history {
		t.Logf("Archival %d: ID=%s, Type=%s, Records=%d, Status=%s",
			i+1, report.ArchivalID, report.TriggerType, report.RecordsArchived, report.ArchivalStatus)
	}
}

// Helper functions

func setupTestDB(t *testing.T) *sql.DB {
	// Try to connect to test database
	// This will use environment variables or default connection string
	connStr := "postgres://user:password@localhost:5432/ipo_db?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Logf("Failed to connect to database: %v", err)
		return nil
	}

	// Test connection
	err = db.Ping()
	if err != nil {
		t.Logf("Failed to ping database: %v", err)
		db.Close()
		return nil
	}

	return db
}

func createTestIPO(t *testing.T, db *sql.DB, companyCode string) string {
	ipoID := uuid.New().String()
	stockID := "STOCK-" + companyCode
	query := `
		INSERT INTO ipo_list (id, stock_id, name, company_code, registrar, status, open_date, close_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (company_code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`

	now := time.Now()
	err := db.QueryRow(query,
		ipoID,
		stockID,
		"Test IPO "+companyCode,
		companyCode,
		"Test Registrar",
		"CLOSED",
		now.AddDate(0, -1, 0),
		now.AddDate(0, 0, -7),
	).Scan(&ipoID)

	if err != nil {
		t.Logf("Failed to create test IPO: %v", err)
	}

	return ipoID
}

func insertTestHistoryEntry(t *testing.T, db *sql.DB, entry *models.GMPPriceHistoryEntry) error {
	query := `
		INSERT INTO gmp_price_history (
			id, ipo_id, company_code, record_date, ipo_price, gmp_value,
			estimated_listing, listing_percent, estimated_profit,
			subscription_status, sub2_sauda, last_updated, data_source,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	_, err := db.Exec(query,
		entry.ID,
		entry.IPOID,
		entry.CompanyCode,
		entry.RecordDate,
		entry.IPOPrice,
		entry.GMPValue,
		entry.EstimatedListing,
		entry.ListingPercent,
		entry.EstimatedProfit,
		entry.SubscriptionStatus,
		entry.Sub2Sauda,
		entry.LastUpdated,
		entry.DataSource,
		entry.CreatedAt,
		entry.UpdatedAt,
	)

	return err
}

func cleanupTestData(t *testing.T, db *sql.DB) {
	// Clean up test data
	queries := []string{
		"DELETE FROM gmp_price_history WHERE company_code LIKE 'TEST-%'",
		"DELETE FROM gmp_price_history_archive WHERE company_code LIKE 'TEST-%'",
		"DELETE FROM ipo_list WHERE company_code LIKE 'TEST-%'",
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			t.Logf("Cleanup warning: %v", err)
		}
	}
}
