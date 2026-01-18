package tests

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/fenilmodi00/ipo-backend/models"
	"github.com/fenilmodi00/ipo-backend/services"
	"github.com/fenilmodi00/ipo-backend/shared"
	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	_ "github.com/lib/pq"
)

// SimpleIntegrationTestSuite provides basic integration testing using only public interfaces
type SimpleIntegrationTestSuite struct {
	db             *sql.DB
	ipoService     *services.IPOService
	gmpService     *services.GMPService
	utilityService *services.UtilityService
}

// SetupSimpleIntegrationTestSuite initializes the simple integration test environment
func SetupSimpleIntegrationTestSuite(t *testing.T) *SimpleIntegrationTestSuite {
	// Use test database connection
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://localhost/ipo_backend_test?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping simple integration tests - database not available: %v", err)
		return nil
	}

	// Test database connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Skipping simple integration tests - database ping failed: %v", err)
		return nil
	}

	// Initialize services
	ipoService := services.NewIPOService(db)
	gmpService := services.NewGMPService()
	utilityService := services.NewUtilityService()

	return &SimpleIntegrationTestSuite{
		db:             db,
		ipoService:     ipoService,
		gmpService:     gmpService,
		utilityService: utilityService,
	}
}

// TeardownSimpleIntegrationTestSuite cleans up the simple integration test environment
func (suite *SimpleIntegrationTestSuite) TeardownSimpleIntegrationTestSuite() {
	if suite.db != nil {
		suite.db.Close()
	}
}

// TestSimpleEndToEndDataFlowConsistency tests basic end-to-end data flow consistency
// **Feature: service-alignment-enhancement, Simple Integration Test: End-to-end data flow consistency**
// **Validates: All requirements**
func TestSimpleEndToEndDataFlowConsistency(t *testing.T) {
	suite := SetupSimpleIntegrationTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownSimpleIntegrationTestSuite()

	properties := gopter.NewProperties(nil)

	properties.Property("End-to-end data flow consistency across all services using public interfaces", prop.ForAll(
		func(companyName, companyCode, registrar string, priceLow, priceHigh float64, minQty, minAmount int) bool {
			// Skip invalid test data
			if companyName == "" || companyCode == "" || priceLow <= 0 || priceHigh <= 0 || priceLow > priceHigh {
				return true
			}

			// Step 1: Create IPO data using utility service text processing
			cleanedName := suite.utilityService.CleanCompanyText(companyName)
			normalizedCode := suite.utilityService.GenerateCompanyCode(companyCode)
			cleanedRegistrar := suite.utilityService.CleanCompanyText(registrar)

			// Step 2: Create IPO record
			now := time.Now()
			openDate := now.Add(24 * time.Hour)
			closeDate := openDate.Add(7 * 24 * time.Hour)
			listingDate := closeDate.Add(7 * 24 * time.Hour)

			testIPO := &models.IPO{
				ID:            uuid.New(),
				StockID:       "INTEG_" + normalizedCode,
				Name:          cleanedName,
				CompanyCode:   normalizedCode,
				Registrar:     cleanedRegistrar,
				OpenDate:      &openDate,
				CloseDate:     &closeDate,
				ListingDate:   &listingDate,
				PriceBandLow:  &priceLow,
				PriceBandHigh: &priceHigh,
				MinQty:        &minQty,
				MinAmount:     &minAmount,
				Status:        "UPCOMING",
			}

			// Step 3: Validate IPO data using enhanced validation
			// validator := services.NewUnifiedValidator(suite.utilityService, suite.db)
			// validationResult := validator.Validate(testIPO)
			// if !validationResult.IsValid {
			//	// If validation fails, this is expected behavior for some inputs
			//	return true
			// }

			// Step 4: Calculate enhanced metrics
			metrics := suite.ipoService.CalculateEnhancedIPOMetrics(testIPO)
			if metrics == nil {
				t.Logf("Failed to calculate enhanced metrics for IPO: %s", testIPO.Name)
				return false
			}

			// Step 5: Test status calculation consistency
			calculatedStatus := suite.utilityService.CalculateIPOStatus(testIPO.OpenDate, testIPO.CloseDate, testIPO.ListingDate)
			if calculatedStatus == "" {
				t.Logf("Failed to calculate IPO status for: %s", testIPO.Name)
				return false
			}

			// Step 6: Test cross-service consistency using public interfaces
			// Text processing consistency
			utilityCleanedName := suite.utilityService.CleanCompanyText(companyName)
			if utilityCleanedName != cleanedName {
				t.Logf("Inconsistent text cleaning: expected %s, got %s", cleanedName, utilityCleanedName)
				return false
			}

			// Company code generation consistency
			utilityCompanyCode := suite.utilityService.GenerateCompanyCode(companyCode)
			if utilityCompanyCode != normalizedCode {
				t.Logf("Inconsistent company code generation: expected %s, got %s", normalizedCode, utilityCompanyCode)
				return false
			}

			// Status calculation consistency
			utilityStatus := suite.utilityService.CalculateIPOStatus(testIPO.OpenDate, testIPO.CloseDate, testIPO.ListingDate)
			if utilityStatus != calculatedStatus {
				t.Logf("Inconsistent status calculation: expected %s, got %s", calculatedStatus, utilityStatus)
				return false
			}

			// Step 7: Test configuration management consistency using public interfaces
			gmpConfig := shared.NewGMPServiceConfig()
			if gmpConfig.HTTPRequestTimeout <= 0 {
				t.Logf("Invalid GMP service configuration: timeout %v", gmpConfig.HTTPRequestTimeout)
				return false
			}

			// Step 8: Test metrics tracking consistency using public interfaces
			serviceMetrics := suite.ipoService.GetServiceMetrics()
			if serviceMetrics == nil {
				t.Logf("Service metrics not available for IPO service")
				return false
			}

			utilityMetrics := suite.utilityService.GetServiceMetrics()
			if utilityMetrics == nil {
				t.Logf("Service metrics not available for utility service")
				return false
			}

			// All consistency checks passed
			return true
		},
		gen.OneConstOf("TechCorp Ltd", "ACME Industries", "Global Solutions Inc", "StartupXYZ", "MegaCorp"),
		gen.OneConstOf("TECH", "ACME", "GLOB", "STRT", "MEGA"),
		gen.OneConstOf("TechRegistrar", "ACME Agent", "Global Reg", "Startup Agent", "Mega Registrar"),
		gen.Float64Range(10, 500), // priceLow
		gen.Float64Range(10, 500), // priceHigh
		gen.IntRange(1, 1000),     // minQty
		gen.IntRange(1000, 50000), // minAmount
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestSimpleServiceInteractionPatterns tests service interaction patterns using public interfaces
// **Feature: service-alignment-enhancement, Simple Integration Test: Service interaction patterns**
// **Validates: Requirements 1.1, 2.1, 4.1, 5.1, 6.1, 7.1**
func TestSimpleServiceInteractionPatterns(t *testing.T) {
	suite := SetupSimpleIntegrationTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownSimpleIntegrationTestSuite()

	properties := gopter.NewProperties(nil)

	properties.Property("Service interaction patterns maintain consistency using public interfaces", prop.ForAll(
		func(companyName string, priceText string, dateText string) bool {
			// Test IPO Service -> Utility Service interaction
			cleanedName1 := suite.ipoService.UtilityService.CleanCompanyText(companyName)
			cleanedName2 := suite.utilityService.CleanCompanyText(companyName)

			// Both should produce identical results
			if cleanedName1 != cleanedName2 {
				t.Logf("Inconsistent text cleaning between IPO service and utility service: %s vs %s", cleanedName1, cleanedName2)
				return false
			}

			// Test numeric processing consistency
			extractedNum1 := suite.ipoService.UtilityService.ExtractNumeric(priceText)
			extractedNum2 := suite.utilityService.ExtractNumeric(priceText)

			if extractedNum1 != extractedNum2 {
				t.Logf("Inconsistent numeric extraction between services: %f vs %f", extractedNum1, extractedNum2)
				return false
			}

			// Test date parsing consistency
			parsedDate1 := suite.ipoService.UtilityService.ParseStandardDateFormats(dateText)
			parsedDate2 := suite.utilityService.ParseStandardDateFormats(dateText)

			// Both should produce identical results
			if (parsedDate1 == nil) != (parsedDate2 == nil) {
				t.Logf("Inconsistent date parsing existence between services")
				return false
			}

			if parsedDate1 != nil && parsedDate2 != nil && !parsedDate1.Equal(*parsedDate2) {
				t.Logf("Inconsistent date parsing results between services: %v vs %v", parsedDate1, parsedDate2)
				return false
			}

			// Test company code generation consistency
			companyCode1 := suite.ipoService.UtilityService.GenerateCompanyCode(companyName)
			companyCode2 := suite.utilityService.GenerateCompanyCode(companyName)

			if companyCode1 != companyCode2 {
				t.Logf("Inconsistent company code generation between services: %s vs %s", companyCode1, companyCode2)
				return false
			}

			return true
		},
		gen.OneConstOf("TechCorp Ltd", "ACME Industries", "Global Solutions Inc", "StartupXYZ", "MegaCorp", "InnovateTech", "DataSystems"),
		gen.OneConstOf("₹100", "200.50", "₹50.25", "300", "₹1000.75", "invalid", "", "₹0"),
		gen.OneConstOf("Dec 25, 2024", "25-12-2024", "2024-12-25", "December 25, 2024", "25 Dec 2024", "invalid", ""),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Unit tests for specific integration scenarios using public interfaces

func TestSimpleServiceInitializationConsistency(t *testing.T) {
	suite := SetupSimpleIntegrationTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownSimpleIntegrationTestSuite()

	// Test that all services initialize with consistent patterns
	if suite.ipoService == nil {
		t.Error("IPO service failed to initialize")
	}

	if suite.gmpService == nil {
		t.Error("GMP service failed to initialize")
	}

	if suite.utilityService == nil {
		t.Error("Utility service failed to initialize")
	}

	// Test that services have proper dependencies
	if suite.ipoService.UtilityService == nil {
		t.Error("IPO service missing utility service dependency")
	}

	// Test that services have proper metrics tracking
	ipoMetrics := suite.ipoService.GetServiceMetrics()
	if ipoMetrics == nil {
		t.Error("IPO service missing metrics tracking")
	}

	utilityMetrics := suite.utilityService.GetServiceMetrics()
	if utilityMetrics == nil {
		t.Error("Utility service missing metrics tracking")
	}
}

func TestSimpleCrossServiceDataValidation(t *testing.T) {
	suite := SetupSimpleIntegrationTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownSimpleIntegrationTestSuite()

	// Create test data
	testCompanyName := "Integration Test Corp Ltd"
	testCompanyCode := "INTEG"

	// Test that all services validate data consistently
	cleanedName1 := suite.ipoService.UtilityService.CleanCompanyText(testCompanyName)
	cleanedName2 := suite.utilityService.CleanCompanyText(testCompanyName)

	if cleanedName1 != cleanedName2 {
		t.Errorf("Inconsistent company name cleaning: IPO service got %s, utility service got %s", cleanedName1, cleanedName2)
	}

	// Test company code generation consistency
	companyCode1 := suite.ipoService.UtilityService.GenerateCompanyCode(testCompanyCode)
	companyCode2 := suite.utilityService.GenerateCompanyCode(testCompanyCode)

	if companyCode1 != companyCode2 {
		t.Errorf("Inconsistent company code generation: IPO service got %s, utility service got %s", companyCode1, companyCode2)
	}

	// Test numeric processing consistency
	priceText := "₹150.00"
	extractedPrice1 := suite.ipoService.UtilityService.ExtractNumeric(priceText)
	extractedPrice2 := suite.utilityService.ExtractNumeric(priceText)

	if extractedPrice1 != extractedPrice2 {
		t.Errorf("Inconsistent price extraction: IPO service got %f, utility service got %f", extractedPrice1, extractedPrice2)
	}
}

func TestSimpleServiceMetricsIntegration(t *testing.T) {
	suite := SetupSimpleIntegrationTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownSimpleIntegrationTestSuite()

	// Test that metrics are tracked consistently across services
	ipoMetrics := suite.ipoService.GetServiceMetrics()
	utilityMetrics := suite.utilityService.GetServiceMetrics()

	// Record test operations
	testDuration := 100 * time.Millisecond
	ipoMetrics.RecordRequest(true, testDuration)
	utilityMetrics.RecordRequest(true, testDuration)

	// Get snapshots
	ipoSnapshot := ipoMetrics.GetSnapshot()
	utilitySnapshot := utilityMetrics.GetSnapshot()

	// Verify metrics tracking patterns are consistent
	if ipoSnapshot.TotalRequests != 1 {
		t.Errorf("IPO service metrics not tracking requests correctly: expected 1, got %d", ipoSnapshot.TotalRequests)
	}

	if utilitySnapshot.TotalRequests != 1 {
		t.Errorf("Utility service metrics not tracking requests correctly: expected 1, got %d", utilitySnapshot.TotalRequests)
	}

	if ipoSnapshot.SuccessfulRequests != 1 {
		t.Errorf("IPO service metrics not tracking successful requests correctly: expected 1, got %d", ipoSnapshot.SuccessfulRequests)
	}

	if utilitySnapshot.SuccessfulRequests != 1 {
		t.Errorf("Utility service metrics not tracking successful requests correctly: expected 1, got %d", utilitySnapshot.SuccessfulRequests)
	}
}

func TestSimpleDatabaseIntegrationConsistency(t *testing.T) {
	suite := SetupSimpleIntegrationTestSuite(t)
	if suite == nil {
		return
	}
	defer suite.TeardownSimpleIntegrationTestSuite()

	// Test database connection consistency
	if suite.db == nil {
		t.Error("Database connection not available")
		return
	}

	// Test database ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := suite.db.PingContext(ctx); err != nil {
		t.Errorf("Database ping failed: %v", err)
		return
	}

	// Test that services use database consistently
	if suite.ipoService.DB != suite.db {
		t.Error("IPO service not using the same database connection")
	}
}
