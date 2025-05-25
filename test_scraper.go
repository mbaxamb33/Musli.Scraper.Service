package main

// import (
// 	"context"
// 	"encoding/json"
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
// 	// Initialize logger
// 	logger, _ := zap.NewDevelopment()
// 	defer logger.Sync()

// 	// Load config
// 	cfg, err := config.LoadConfig()
// 	if err != nil {
// 		log.Fatal("Config error:", err)
// 	}

// 	// Create scraper engine
// 	engine, err := scraper.NewEngine(cfg, logger)
// 	if err != nil {
// 		log.Fatal("Engine error:", err)
// 	}
// 	defer engine.Close()

// 	// Test with multi-page crawling
// 	url := "https://www.druidai.com"

// 	options := models.ScrapingOptions{
// 		WaitForJS:      false,
// 		Timeout:        30 * time.Second,
// 		ScrollToBottom: false,
// 		Depth:          2,    // Crawl 2 levels deep
// 		SameDomainOnly: true, // Stay on same domain
// 		MaxPages:       20,   // Limit to 10 pages max
// 		ExcludePatterns: []string{
// 			`\.(pdf|doc|docx|zip)$`, // Skip file downloads
// 			`/admin/.*`,             // Skip admin pages
// 		},
// 	}

// 	fmt.Printf("Scraping: %s (depth: %d)\n", url, options.Depth)

// 	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // Increased timeout for multi-page
// 	defer cancel()

// 	// Scrape the page(s)
// 	results, err := engine.ScrapePage(ctx, url, options)
// 	if err != nil {
// 		log.Fatal("Scraping error:", err)
// 	}

// 	// Output modules as JSON
// 	modulesJSON, err := json.MarshalIndent(results.ModulePairs, "", "  ")
// 	if err != nil {
// 		log.Fatal("JSON marshal error:", err)
// 	}

// 	fmt.Println(string(modulesJSON))

// 	// Summary to stderr
// 	fmt.Fprintf(os.Stderr, "Extracted %d modules from %s\n", len(results.ModulePairs), results.URL)

// 	// Crawl stats if multi-page
// 	if results.CrawlResults != nil {
// 		fmt.Fprintf(os.Stderr, "Crawl stats: %d pages scraped, %d total found, %v duration\n",
// 			results.CrawlResults.TotalPagesScraped,
// 			results.CrawlResults.TotalPagesFound,
// 			results.CrawlResults.CrawlDuration)

// 		// Show pages scraped
// 		fmt.Fprintf(os.Stderr, "Pages scraped:\n")
// 		for _, page := range results.CrawlResults.PagesSummary {
// 			status := "✓"
// 			if !page.Success {
// 				status = "✗"
// 			}
// 			fmt.Fprintf(os.Stderr, "  %s [depth %d] %s (%d modules)\n",
// 				status, page.Depth, page.URL, page.ModuleCount)
// 		}
// 	}
// }
