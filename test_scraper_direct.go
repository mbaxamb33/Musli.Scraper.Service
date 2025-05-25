// test_scraper_direct.go - Test the scraper engine directly
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/internal/scraper"
	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Config error:", err)
	}

	// Create scraper engine
	engine, err := scraper.NewEngine(cfg, logger)
	if err != nil {
		log.Fatal("Engine error:", err)
	}
	defer engine.Close()

	// Test with a simple webpage
	url := "https://httpbin.org/html"

	options := models.ScrapingOptions{
		WaitForJS:      false,
		Timeout:        30 * time.Second,
		ScrollToBottom: false,
	}

	fmt.Printf("🧪 Testing scraper engine directly\n")
	fmt.Printf("URL: %s\n", url)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Scrape the page
	results, err := engine.ScrapePage(ctx, url, options)
	if err != nil {
		fmt.Printf("❌ Scraping failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Scraping successful!\n")
	fmt.Printf("Title: %s\n", results.Title)
	fmt.Printf("Modules extracted: %d\n", len(results.ModulePairs))
	fmt.Printf("Processing time: %v\n", results.ProcessingStats.ProcessingTime)

	// Show first module if available
	if len(results.ModulePairs) > 0 {
		fmt.Printf("First module title: %s\n", results.ModulePairs[0].Title)
		fmt.Printf("First module content length: %d\n", len(results.ModulePairs[0].Content))
	}

	// Test the problematic URL from your jobs
	fmt.Printf("\n🧪 Testing example.com (from processing jobs)\n")

	results2, err := engine.ScrapePage(ctx, "https://example.com", options)
	if err != nil {
		fmt.Printf("❌ Example.com scraping failed: %v\n", err)
	} else {
		fmt.Printf("✅ Example.com scraping successful!\n")
		fmt.Printf("Modules: %d\n", len(results2.ModulePairs))
	}
}
