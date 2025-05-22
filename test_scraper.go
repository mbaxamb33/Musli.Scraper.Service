package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	url := "https://www.druidai.com/solutions/customer-support-ai-agents"

	options := models.ScrapingOptions{
		WaitForJS:      false,
		Timeout:        30 * time.Second,
		ScrollToBottom: false,
	}

	fmt.Printf("Scraping: %s\n", url)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Scrape the page
	results, err := engine.ScrapePage(ctx, url, options)
	if err != nil {
		log.Fatal("Scraping error:", err)
	}

	// Output only modules as JSON
	modulesJSON, err := json.MarshalIndent(results.ModulePairs, "", "  ")
	if err != nil {
		log.Fatal("JSON marshal error:", err)
	}

	fmt.Println(string(modulesJSON))

	// Optional: Summary to stderr so it doesn't interfere with JSON output
	fmt.Fprintf(os.Stderr, "Extracted %d modules from %s\n", len(results.ModulePairs), results.URL)
}
