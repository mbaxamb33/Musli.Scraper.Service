// // test_scraper_fixed.go - Test with better error handling and timeouts
package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"time"

// 	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
// 	"github.com/mbaxamb3/nusli/scraper-service/internal/scraper"
// 	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
// 	"go.uber.org/zap"
// )

// func main() {
// 	// Initialize logger
// 	logger, _ := zap.NewDevelopment()
// 	defer logger.Sync()

// 	// Load config
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Fatal("Config error:", err)
// 	}

// 	// Override timeouts to be more aggressive
// 	cfg.BrowserTimeout = 10 * time.Second
// 	cfg.PageLoadWait = 1 * time.Second

// 	// Create scraper engine
// 	engine, err := scraper.NewEngine(cfg, logger)
// 	if err != nil {
// 		log.Fatal("Engine error:", err)
// 	}
// 	defer engine.Close()

// 	// Test with a simple, fast webpage first
// 	testURLs := []string{
// 		"data:text/html,<html><body><h1>Test Page</h1><p>This is a test</p></body></html>",
// 		"https://httpbin.org/html",
// 		"https://example.com",
// 	}

// 	for i, url := range testURLs {
// 		fmt.Printf("\n🧪 Test %d: %s\n", i+1, url)

// 		options := models.ScrapingOptions{
// 			WaitForJS:      false,
// 			Timeout:        10 * time.Second,
// 			ScrollToBottom: false,
// 		}

// 		// Use a shorter timeout context
// 		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

// 		// Scrape the page
// 		start := time.Now()
// 		results, err := engine.ScrapePage(ctx, url, options)
// 		duration := time.Since(start)

// 		if err != nil {
// 			fmt.Printf("❌ Failed after %v: %v\n", duration, err)
// 		} else {
// 			fmt.Printf("✅ Success in %v!\n", duration)
// 			fmt.Printf("   Title: %s\n", results.Title)
// 			fmt.Printf("   Modules: %d\n", len(results.ModulePairs))
// 			if len(results.ModulePairs) > 0 {
// 				fmt.Printf("   First module: %s\n", results.ModulePairs[0].Title)
// 			}
// 		}

// 		cancel()

// 		// If first test fails, don't continue
// 		if err != nil && i == 0 {
// 			fmt.Printf("❌ Basic test failed, stopping\n")
// 			break
// 		}
// 	}
// }
