// // test_druidai_engine.go - Direct test of scraping DruidAI website
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
// 	fmt.Println("🧪 DruidAI Website Scraping Test")
// 	fmt.Println("================================")

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

// 	// Configure for better success with modern websites
// 	cfg.BrowserHeadless = true
// 	cfg.BrowserTimeout = 60 * time.Second
// 	cfg.PageLoadWait = 3 * time.Second

// 	// Create scraper engine
// 	fmt.Println("Creating scraper engine...")
// 	engine, err := scraper.NewEngine(cfg, logger)
// 	if err != nil {
// 		log.Fatal("Failed to create scraper engine:", err)
// 	}
// 	defer engine.Close()

// 	fmt.Println("✅ Scraper engine created successfully")

// 	// Test DruidAI website
// 	fmt.Println("\n🔍 Testing DruidAI website...")
// 	testURL := "https://www.druidai.com/"

// 	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
// 	defer cancel()

// 	// Configure scraping options for modern website
// 	options := models.ScrapingOptions{
// 		WaitForJS:      true, // DruidAI likely uses JavaScript
// 		Timeout:        60 * time.Second,
// 		ScrollToBottom: false, // Keep it simple for now
// 		Screenshot:     false,
// 	}

// 	fmt.Printf("Target URL: %s\n", testURL)
// 	fmt.Printf("Options: WaitForJS=%t, Timeout=%v\n", options.WaitForJS, options.Timeout)

// 	start := time.Now()
// 	fmt.Println("\n🚀 Starting scraping process...")

// 	results, err := engine.ScrapePage(ctx, testURL, options)
// 	duration := time.Since(start)

// 	if err != nil {
// 		fmt.Printf("❌ Scraping failed after %v: %v\n", duration, err)

// 		// Try with different options if it failed
// 		fmt.Println("\n🔄 Retrying with simpler options...")
// 		simpleOptions := models.ScrapingOptions{
// 			WaitForJS: false,
// 			Timeout:   30 * time.Second,
// 		}

// 		ctx2, cancel2 := context.WithTimeout(context.Background(), 45*time.Second)
// 		defer cancel2()

// 		results, err = engine.ScrapePage(ctx2, testURL, simpleOptions)
// 		if err != nil {
// 			fmt.Printf("❌ Second attempt also failed: %v\n", err)
// 			return
// 		}
// 		fmt.Println("✅ Second attempt succeeded!")
// 	}

// 	fmt.Printf("✅ Scraping completed in %v\n", duration)

// 	// Display results
// 	fmt.Println("\n📊 Scraping Results:")
// 	fmt.Printf("━━━━━━━━━━━━━━━━━━━━\n")
// 	fmt.Printf("Title: %s\n", results.Title)
// 	fmt.Printf("URL: %s\n", results.URL)
// 	fmt.Printf("Content Length: %d chars\n", results.ProcessingStats.ContentLength)
// 	fmt.Printf("Processing Time: %v\n", results.ProcessingStats.ProcessingTime)
// 	fmt.Printf("Modules Extracted: %d\n", len(results.ModulePairs))

// 	// Display module information
// 	if len(results.ModulePairs) > 0 {
// 		fmt.Println("\n📄 Extracted Content Modules:")
// 		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

// 		for i, module := range results.ModulePairs {
// 			if i >= 10 { // Limit to first 10 modules
// 				fmt.Printf("... and %d more modules\n", len(results.ModulePairs)-10)
// 				break
// 			}

// 			contentPreview := module.Content
// 			if len(contentPreview) > 150 {
// 				contentPreview = contentPreview[:150] + "..."
// 			}

// 			fmt.Printf("\n%d. %s (Level %d)\n", i+1, module.Title, module.Level)
// 			fmt.Printf("   Content: %s\n", contentPreview)
// 		}
// 	} else {
// 		fmt.Println("⚠️  No content modules were extracted")
// 		fmt.Println("This might indicate:")
// 		fmt.Println("  - The website requires JavaScript to load content")
// 		fmt.Println("  - The content structure is not recognized by the extractor")
// 		fmt.Println("  - The website is blocking automated access")
// 	}

// 	// Additional analysis
// 	fmt.Println("\n🔍 Content Analysis:")
// 	fmt.Printf("━━━━━━━━━━━━━━━━━━━━\n")

// 	totalContentLength := 0
// 	levelCounts := make(map[int]int)

// 	for _, module := range results.ModulePairs {
// 		totalContentLength += len(module.Content)
// 		levelCounts[module.Level]++
// 	}

// 	fmt.Printf("Total extracted text: %d characters\n", totalContentLength)
// 	fmt.Printf("Module levels distribution:\n")
// 	for level := 1; level <= 6; level++ {
// 		if count := levelCounts[level]; count > 0 {
// 			fmt.Printf("  - Level %d headers: %d\n", level, count)
// 		}
// 	}

// 	// Success/failure assessment
// 	fmt.Println("\n🎯 Assessment:")
// 	fmt.Printf("━━━━━━━━━━━━━━━\n")

// 	if len(results.ModulePairs) > 0 && totalContentLength > 500 {
// 		fmt.Println("✅ SUCCESS: Content extraction appears successful")
// 		fmt.Printf("   - Extracted %d modules with %d total characters\n", len(results.ModulePairs), totalContentLength)
// 		fmt.Println("   - This suggests the scraper can handle the DruidAI website")
// 	} else if len(results.ModulePairs) > 0 {
// 		fmt.Println("⚠️  PARTIAL: Some content extracted but limited quantity")
// 		fmt.Println("   - May need different scraping options or longer wait times")
// 	} else {
// 		fmt.Println("❌ LIMITED: No structured content extracted")
// 		fmt.Println("   - The website might require different scraping approach")
// 		fmt.Println("   - Consider enabling JavaScript waiting or different selectors")
// 	}

// 	fmt.Printf("\n🎉 DruidAI scraping test completed!\n")
// 	fmt.Printf("Duration: %v\n", duration)
// }
