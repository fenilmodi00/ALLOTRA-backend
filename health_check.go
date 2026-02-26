//go:build ignore

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fenilmodi00/ipo-backend/config"
	"github.com/fenilmodi00/ipo-backend/database"
	"github.com/fenilmodi00/ipo-backend/services"
)

func main() {
	cfg := config.LoadConfig()

	fmt.Printf("🏥 IPO Scraper Health Check - %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 50))

	// Quick tests
	healthScore := 0
	totalTests := 4

	// Test 1: Chittorgarh API
	fmt.Print("📡 Chittorgarh API: ")
	chittorgarh := services.NewChittorgarhIPOScrapingService(nil)
	if items, err := chittorgarh.FetchAvailableIPOList(); err != nil {
		fmt.Printf("❌ FAILED (%v)\n", err)
	} else {
		fmt.Printf("✅ OK (%d IPOs)\n", len(items))
		healthScore++
	}

	// Test 2: GMP Service
	fmt.Print("📈 GMP Service: ")
	gmp := services.NewGMPService()
	if gmpData, err := gmp.FetchGMPData(); err != nil {
		fmt.Printf("❌ FAILED (%v)\n", err)
	} else {
		fmt.Printf("✅ OK (%d records)\n", len(gmpData))
		healthScore++
	}

	// Test 3: Database
	fmt.Print("🗄️  Database: ")
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		fmt.Printf("❌ FAILED (%v)\n", err)
	} else {
		fmt.Println("✅ OK")
		healthScore++
		database.Close()
	}

	// Test 4: Database Data
	fmt.Print("📊 Database Data: ")
	if err := database.Connect(cfg.DatabaseURL); err != nil {
		fmt.Printf("❌ FAILED (%v)\n", err)
	} else {
		ipoService := services.NewIPOService(database.DB)
		ctx := context.Background()
		if ipos, err := ipoService.GetActiveIPOs(ctx); err != nil {
			fmt.Printf("❌ FAILED (%v)\n", err)
		} else {
			fmt.Printf("✅ OK (%d active IPOs)\n", len(ipos))
			healthScore++
		}
		database.Close()
	}

	// Overall health
	fmt.Println(strings.Repeat("-", 50))
	healthPercent := float64(healthScore) / float64(totalTests) * 100

	if healthScore == totalTests {
		fmt.Printf("🎉 SYSTEM HEALTHY: %d/%d tests passed (%.0f%%)\n", healthScore, totalTests, healthPercent)
	} else if healthScore >= totalTests/2 {
		fmt.Printf("⚠️  SYSTEM DEGRADED: %d/%d tests passed (%.0f%%)\n", healthScore, totalTests, healthPercent)
	} else {
		fmt.Printf("❌ SYSTEM UNHEALTHY: %d/%d tests passed (%.0f%%)\n", healthScore, totalTests, healthPercent)
	}

	fmt.Printf("⏰ Check completed at: %s\n", time.Now().Format("15:04:05"))
}
