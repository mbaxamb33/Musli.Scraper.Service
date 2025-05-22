// internal/scraper/engine.go
package scraper

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
	"go.uber.org/zap"
)

// Engine represents the main scraping engine
type Engine struct {
	config  *config.Config
	logger  *zap.Logger
	browser *rod.Browser
}

// NewEngine creates a new scraping engine
func NewEngine(cfg *config.Config, logger *zap.Logger) (*Engine, error) {
	// Setup browser launcher
	launcher := launcher.New().
		Headless(cfg.BrowserHeadless).
		UserDataDir("").
		Set("user-agent", cfg.BrowserUserAgent)

	if cfg.BrowserIncognito {
		launcher = launcher.Set("incognito")
	}

	// Launch browser
	browser := rod.New().ControlURL(launcher.MustLaunch())

	// Connect to browser
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	engine := &Engine{
		config:  cfg,
		logger:  logger,
		browser: browser,
	}

	logger.Info("Scraping engine initialized successfully")
	return engine, nil
}

// ScrapeURL performs intelligent scraping of a URL
func (e *Engine) ScrapeURL(ctx context.Context, url string, options models.ScrapingOptions) (*models.ScrapingResults, error) {
	startTime := time.Now()

	e.logger.Info("Starting scrape",
		zap.String("url", url),
		zap.Any("options", options))

	// Create new page
	page := e.browser.MustPage()
	defer page.Close()

	// Configure page
	if err := e.configurePage(page, options); err != nil {
		return nil, fmt.Errorf("failed to configure page: %w", err)
	}

	// Navigate to URL with timeout
	navigateCtx, cancel := context.WithTimeout(ctx, e.config.ContentTimeout)
	defer cancel()

	if err := page.Context(navigateCtx).Navigate(url); err != nil {
		return nil, fmt.Errorf("failed to navigate to URL: %w", err)
	}

	// Wait for page to load
	if err := e.waitForPageLoad(page, options); err != nil {
		return nil, fmt.Errorf("page load timeout: %w", err)
	}

	// Extract page metadata
	metadata, err := e.extractPageMetadata(page)
	if err != nil {
		e.logger.Warn("Failed to extract page metadata", zap.Error(err))
		metadata = &models.PageMetadata{}
	}

	// Extract content using basic module detection
	modules, err := e.extractBasicModules(page, url)
	if err != nil {
		return nil, fmt.Errorf("failed to extract modules: %w", err)
	}

	// Calculate processing stats
	processingTime := time.Since(startTime)
	stats := e.calculateProcessingStats(page, processingTime, len(modules))

	results := &models.ScrapingResults{
		URL:             url,
		Title:           metadata.Title,
		Description:     metadata.Description,
		Language:        metadata.Language,
		ModulePairs:     modules,
		Metadata:        *metadata,
		ProcessingStats: stats,
	}

	e.logger.Info("Scrape completed successfully",
		zap.String("url", url),
		zap.Int("modules_extracted", len(modules)),
		zap.Duration("processing_time", processingTime))

	return results, nil
}

// configurePage sets up the page with the given options
func (e *Engine) configurePage(page *rod.Page, options models.ScrapingOptions) error {
	// Set viewport
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  int(e.config.BrowserViewportWidth),
		Height: int(e.config.BrowserViewportHeight),
	}); err != nil {
		return fmt.Errorf("failed to set viewport: %w", err)
	}

	// Set user agent if specified
	if options.UserAgent != "" {
		if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: options.UserAgent,
		}); err != nil {
			return fmt.Errorf("failed to set user agent: %w", err)
		}
	}

	return nil
}

// waitForPageLoad waits for the page to fully load
func (e *Engine) waitForPageLoad(page *rod.Page, options models.ScrapingOptions) error {
	// Wait for DOM content loaded
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("DOM load timeout: %w", err)
	}

	// Wait for network idle if JavaScript is enabled
	if options.WaitForJS {
		if err := page.WaitIdle(e.config.PageLoadWait); err != nil {
			e.logger.Warn("Network idle timeout", zap.Error(err))
		}
	}

	// Wait for specific selector if provided
	if options.WaitForSelector != "" {
		element, err := page.Timeout(e.config.ContentTimeout).Element(options.WaitForSelector)
		if err != nil {
			return fmt.Errorf("selector wait timeout: %w", err)
		}
		if err := element.WaitVisible(); err != nil {
			return fmt.Errorf("element visibility timeout: %w", err)
		}
	}

	// Scroll to bottom if requested
	if options.ScrollToBottom {
		if err := e.scrollToBottom(page); err != nil {
			e.logger.Warn("Failed to scroll to bottom", zap.Error(err))
		}
	}

	// Additional wait for page load
	time.Sleep(e.config.PageLoadWait)

	return nil
}

// scrollToBottom scrolls the page to the bottom to trigger lazy loading
func (e *Engine) scrollToBottom(page *rod.Page) error {
	// Simple scroll approach
	for i := 0; i < 10; i++ {
		page.MustEval("window.scrollTo(0, document.body.scrollHeight)")
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// extractPageMetadata extracts metadata from the page
func (e *Engine) extractPageMetadata(page *rod.Page) (*models.PageMetadata, error) {
	metadata := &models.PageMetadata{}

	// Extract title
	if title := page.MustEval("document.title || ''").Str(); title != "" {
		metadata.Title = title
	}

	// Extract description
	if desc := page.MustEval(`document.querySelector('meta[name="description"]')?.content || ""`).Str(); desc != "" {
		metadata.Description = desc
	}

	// Extract language
	if lang := page.MustEval(`document.documentElement.lang || ""`).Str(); lang != "" {
		metadata.Language = lang
	}

	// Extract canonical URL
	if canonical := page.MustEval(`document.querySelector('link[rel="canonical"]')?.href || ""`).Str(); canonical != "" {
		metadata.Canonical = canonical
	}

	// Extract OpenGraph data
	if ogTitle := page.MustEval(`document.querySelector('meta[property="og:title"]')?.content || ""`).Str(); ogTitle != "" {
		metadata.OGTitle = ogTitle
	}

	if ogDesc := page.MustEval(`document.querySelector('meta[property="og:description"]')?.content || ""`).Str(); ogDesc != "" {
		metadata.OGDescription = ogDesc
	}

	if ogImage := page.MustEval(`document.querySelector('meta[property="og:image"]')?.content || ""`).Str(); ogImage != "" {
		metadata.OGImage = ogImage
	}

	return metadata, nil
}

// extractBasicModules extracts basic content modules
func (e *Engine) extractBasicModules(page *rod.Page, url string) ([]models.ModuleTitlePair, error) {
	var modules []models.ModuleTitlePair

	// Extract headers and their associated content
	headers, err := page.Elements("h1, h2, h3, h4, h5, h6")
	if err != nil {
		return nil, fmt.Errorf("failed to find headers: %w", err)
	}

	for i, header := range headers {
		// Get header text
		headerText, err := header.Text()
		if err != nil || strings.TrimSpace(headerText) == "" {
			continue
		}

		// Get header level from the tag name
		level := e.getHeaderLevel(header)

		// Get content from the page - simplified approach
		content := e.getContentAfterHeader(page, i)

		// Create module if we have sufficient content
		if len(content) >= e.config.MinContentLength {
			module := models.ModuleTitlePair{
				ID:                 fmt.Sprintf("module_%d_%d", i, time.Now().Unix()),
				Title:              strings.TrimSpace(headerText),
				Content:            content,
				Level:              level,
				ContentType:        models.ContentTypeGeneral,
				InformationDensity: e.analyzeDensity(content),
				SemanticTags:       []string{},
				RelationshipType:   models.RelationshipHierarchical,
				Metadata: models.ModuleMetadata{
					WordCount:   len(strings.Fields(content)),
					ReadingTime: e.calculateReadingTime(content),
				},
				ExtractedAt: time.Now(),
			}
			modules = append(modules, module)
		}
	}

	// If no headers found, extract paragraphs as modules
	if len(modules) == 0 {
		paragraphs, err := page.Elements("p")
		if err == nil {
			for i, p := range paragraphs {
				text, err := p.Text()
				if err != nil || len(strings.TrimSpace(text)) < e.config.MinContentLength {
					continue
				}

				module := models.ModuleTitlePair{
					ID:                 fmt.Sprintf("paragraph_%d_%d", i, time.Now().Unix()),
					Title:              fmt.Sprintf("Content Section %d", i+1),
					Content:            strings.TrimSpace(text),
					Level:              1,
					ContentType:        models.ContentTypeGeneral,
					InformationDensity: e.analyzeDensity(text),
					SemanticTags:       []string{},
					RelationshipType:   models.RelationshipSequential,
					Metadata: models.ModuleMetadata{
						WordCount:   len(strings.Fields(text)),
						ReadingTime: e.calculateReadingTime(text),
					},
					ExtractedAt: time.Now(),
				}
				modules = append(modules, module)

				// Limit to prevent too many modules
				if len(modules) >= 10 {
					break
				}
			}
		}
	}

	return modules, nil
}

// getHeaderLevel determines the header level (h1=1, h2=2, etc.)
func (e *Engine) getHeaderLevel(header *rod.Element) int {
	// Default to level 1 if we can't determine
	level := 1

	// Try to get the tag name and extract level
	if tagName := header.MustEval("this.tagName.toLowerCase()").Str(); len(tagName) == 2 && tagName[0] == 'h' {
		if l := int(tagName[1] - '0'); l >= 1 && l <= 6 {
			level = l
		}
	}

	return level
}

// getContentAfterHeader gets content that appears after a header
func (e *Engine) getContentAfterHeader(page *rod.Page, headerIndex int) string {
	// Simple approach: get the next few paragraphs from the page
	paragraphs, err := page.Elements("p")
	if err != nil {
		return ""
	}

	// Start from a position that might be after our header
	startIndex := headerIndex
	if startIndex >= len(paragraphs) {
		startIndex = 0
	}

	var contentParts []string
	for i := startIndex; i < len(paragraphs) && len(contentParts) < 3; i++ {
		if text, err := paragraphs[i].Text(); err == nil {
			text = strings.TrimSpace(text)
			if len(text) > 20 { // Only include substantial text
				contentParts = append(contentParts, text)
			}
		}
	}

	return strings.Join(contentParts, "\n\n")
}

// Helper functions
func (e *Engine) analyzeDensity(content string) models.DensityLevel {
	wordCount := len(strings.Fields(content))

	if wordCount < 100 {
		return models.DensityOverview
	} else if wordCount < 500 {
		return models.DensityDetailed
	} else {
		return models.DensityReference
	}
}

func (e *Engine) calculateReadingTime(content string) int {
	wordCount := len(strings.Fields(content))
	// Average reading speed: 200 words per minute
	return (wordCount + 199) / 200
}

// calculateProcessingStats calculates processing statistics
func (e *Engine) calculateProcessingStats(page *rod.Page, processingTime time.Duration, moduleCount int) models.ProcessingStats {
	stats := models.ProcessingStats{
		ProcessingTime:    processingTime,
		ModulesExtracted:  moduleCount,
		JavaScriptEnabled: true,
	}

	// Get basic stats with error handling
	if contentLength := page.MustEval("document.documentElement.outerHTML.length || 0").Num(); contentLength > 0 {
		stats.ContentLength = int(contentLength)
	}

	if imageCount := page.MustEval("document.images ? document.images.length : 0").Num(); imageCount > 0 {
		stats.ImagesFound = int(imageCount)
	}

	if linkCount := page.MustEval("document.links ? document.links.length : 0").Num(); linkCount > 0 {
		stats.LinksFound = int(linkCount)
	}

	return stats
}

// Close closes the scraping engine and browser
func (e *Engine) Close() error {
	if e.browser != nil {
		e.browser.Close()
	}
	e.logger.Info("Scraping engine closed")
	return nil
}
