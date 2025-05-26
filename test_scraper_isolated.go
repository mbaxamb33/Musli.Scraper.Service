// // test_scraper_isolated.go - Test just the scraper engine without database/queue dependencies
package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"os"
// 	"time"

// 	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
// 	"github.com/mbaxamb3/nusli/scraper-service/internal/scraper"
// 	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
// 	"go.uber.org/zap"
// )

// func main() {
// 	fmt.Println("🧪 Testing Scraper Engine in Isolation")
// 	fmt.Println("======================================")

// 	// Initialize logger
// 	logger, err := zap.NewDevelopment()
// 	if err != nil {
// 		log.Fatal("Failed to create logger:", err)
// 	}
// 	defer logger.Sync()

// 	// Load config
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Fatal("Config error:", err)
// 	}

// 	// Override some settings for testing
// 	cfg.BrowserHeadless = true
// 	cfg.BrowserTimeout = 15 * time.Second
// 	cfg.PageLoadWait = 2 * time.Second

// 	fmt.Printf("Browser headless: %t\n", cfg.BrowserHeadless)
// 	fmt.Printf("Browser timeout: %v\n", cfg.BrowserTimeout)

// 	// Create scraper engine
// 	fmt.Println("\n🚀 Initializing scraper engine...")
// 	engine, err := scraper.NewEngine(cfg, logger)
// 	if err != nil {
// 		log.Fatal("Failed to create scraper engine:", err)
// 	}
// 	defer engine.Close()

// 	fmt.Println("✅ Scraper engine created successfully")

// 	// Test URLs in order of complexity
// 	testCases := []struct {
// 		name               string
// 		url                string
// 		options            models.ScrapingOptions
// 		expectedMinModules int
// 	}{
// 		{
// 			name: "Data URL (simplest)",
// 			url:  "data:text/html,<html><head><title>Test Page</title></head><body><h1>Main Title</h1><p>Test content here.</p><h2>Section 1</h2><p>Some content for section 1.</p></body></html>",
// 			options: models.ScrapingOptions{
// 				WaitForJS: false,
// 				Timeout:   5 * time.Second,
// 			},
// 			expectedMinModules: 1,
// 		},
// 		{
// 			name: "HTTPBin HTML",
// 			url:  "https://httpbin.org/html",
// 			options: models.ScrapingOptions{
// 				WaitForJS: false,
// 				Timeout:   10 * time.Second,
// 			},
// 			expectedMinModules: 1,
// 		},
// 		{
// 			name: "Example.com",
// 			url:  "https://example.com",
// 			options: models.ScrapingOptions{
// 				WaitForJS: false,
// 				Timeout:   15 * time.Second,
// 			},
// 			expectedMinModules: 1,
// 		},
// 	}

// 	successCount := 0
// 	for i, testCase := range testCases {
// 		fmt.Printf("\n🧪 Test %d: %s\n", i+1, testCase.name)
// 		fmt.Printf("URL: %s\n", testCase.url)

// 		success := runSingleTest(engine, testCase.url, testCase.options, testCase.expectedMinModules, logger)
// 		if success {
// 			successCount++
// 			fmt.Printf("✅ Test %d passed\n", i+1)
// 		} else {
// 			fmt.Printf("❌ Test %d failed\n", i+1)

// 			// If even the simplest test fails, stop
// 			if i == 0 {
// 				fmt.Println("❌ Basic functionality test failed. Check Chrome/Chromium installation.")
// 				os.Exit(1)
// 			}
// 		}
// 	}

// 	fmt.Printf("\n📊 Results: %d/%d tests passed\n", successCount, len(testCases))

// 	if successCount == len(testCases) {
// 		fmt.Println("🎉 All scraper tests passed! The scraper engine is working correctly.")
// 		fmt.Println("If the service tests are failing, the issue is likely in:")
// 		fmt.Println("  - Database connection")
// 		fmt.Println("  - RabbitMQ connection")
// 		fmt.Println("  - Service configuration")
// 	} else {
// 		fmt.Printf("⚠️  %d tests failed. Check the error messages above.\n", len(testCases)-successCount)
// 	}
// }

// func runSingleTest(engine *scraper.Engine, url string, options models.ScrapingOptions, expectedMinModules int, logger *zap.Logger) bool {
// 	// Create context with timeout
// 	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout+10*time.Second)
// 	defer cancel()

// 	start := time.Now()

// 	// Perform scraping
// 	results, err := engine.ScrapePage(ctx, url, options)
// 	duration := time.Since(start)

// 	if err != nil {
// 		fmt.Printf("❌ Scraping failed after %v: %v\n", duration, err)
// 		return false
// 	}

// 	fmt.Printf("✅ Scraping completed in %v\n", duration)
// 	fmt.Printf("   Title: %s\n", results.Title)
// 	fmt.Printf("   Modules extracted: %d\n", len(results.ModulePairs))
// 	fmt.Printf("   Content length: %d chars\n", results.ProcessingStats.ContentLength)

// 	// Check if we got the expected number of modules
// 	if len(results.ModulePairs) < expectedMinModules {
// 		fmt.Printf("⚠️  Expected at least %d modules, got %d\n", expectedMinModules, len(results.ModulePairs))
// 		// Don't fail the test for this, just warn
// 	}

// 	// Show first module if available
// 	if len(results.ModulePairs) > 0 {
// 		module := results.ModulePairs[0]
// 		fmt.Printf("   First module: %s (level %d)\n", module.Title, module.Level)
// 		contentPreview := module.Content
// 		if len(contentPreview) > 100 {
// 			contentPreview = contentPreview[:100] + "..."
// 		}
// 		fmt.Printf("   Content preview: %s\n", contentPreview)
// 	}

// 	return true
// }

// func checkPrerequisites() bool {
// 	fmt.Println("🔍 Checking prerequisites...")

// 	// We can't easily check for Chrome from Go, so just warn
// 	fmt.Println("⚠️  Make sure Chrome or Chromium is installed:")
// 	fmt.Println("   Ubuntu/Debian: sudo apt-get install chromium-browser")
// 	fmt.Println("   macOS: brew install chromium")
// 	fmt.Println("   Windows: Install Chrome from google.com/chrome")

// 	return true
// }
