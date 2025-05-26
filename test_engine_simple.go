// // test_engine_simple.go - Simple test of the scraper engine
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
// 	fmt.Println("🧪 Simple Scraper Engine Test")
// 	fmt.Println("=============================")

// 	// Create logger
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

// 	// Create scraper engine
// 	fmt.Println("Creating scraper engine...")
// 	engine, err := scraper.NewEngine(cfg, logger)
// 	if err != nil {
// 		log.Fatal("Failed to create scraper engine:", err)
// 	}
// 	defer engine.Close()

// 	fmt.Println("✅ Scraper engine created successfully")

// 	// Test simple HTML
// 	fmt.Println("\n🔍 Testing with simple HTML...")

// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	defer cancel()

// 	options := models.ScrapingOptions{
// 		WaitForJS: false,
// 		Timeout:   15 * time.Second,
// 	}

// 	// Test URL
// 	testURL := "https://example.com"
// 	fmt.Printf("Scraping: %s\n", testURL)

// 	start := time.Now()
// 	results, err := engine.ScrapePage(ctx, testURL, options)
// 	duration := time.Since(start)

// 	if err != nil {
// 		fmt.Printf("❌ Scraping failed after %v: %v\n", duration, err)
// 		return
// 	}

// 	fmt.Printf("✅ Scraping completed in %v\n", duration)
// 	fmt.Printf("Title: %s\n", results.Title)
// 	fmt.Printf("Modules extracted: %d\n", len(results.ModulePairs))
// 	fmt.Printf("Processing time: %v\n", results.ProcessingStats.ProcessingTime)

// 	if len(results.ModulePairs) > 0 {
// 		fmt.Printf("First module: %s\n", results.ModulePairs[0].Title)
// 		content := results.ModulePairs[0].Content
// 		if len(content) > 100 {
// 			content = content[:100] + "..."
// 		}
// 		fmt.Printf("Content preview: %s\n", content)
// 	}

// 	fmt.Println("\n🎉 Engine test completed successfully!")
// }
