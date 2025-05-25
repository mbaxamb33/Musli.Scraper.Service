// internal/scraper/engine.go - Updated with crawling support
package scraper

import (
	"context"
	"fmt"
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
	browser *rod.Browser
	config  *config.Config
	logger  *zap.Logger
}

// NewEngine creates a new scraping engine instance
func NewEngine(cfg *config.Config, logger *zap.Logger) (*Engine, error) {
	// Configure launcher
	l := launcher.New().
		Headless(cfg.BrowserHeadless)

	if cfg.BrowserUserAgent != "" {
		l = l.UserDataDir("")
	}

	// Launch browser
	url, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	// Connect to browser
	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to browser: %w", err)
	}

	logger.Info("Browser launched successfully",
		zap.Bool("headless", cfg.BrowserHeadless),
		zap.String("user_agent", cfg.BrowserUserAgent))

	return &Engine{
		browser: browser,
		config:  cfg,
		logger:  logger,
	}, nil
}

// Close closes the browser and cleans up resources
func (e *Engine) Close() error {
	if e.browser != nil {
		e.logger.Info("Closing browser")
		return e.browser.Close()
	}
	return nil
}

// ScrapePage is the main entry point that handles both single-page and multi-page scraping

func (e *Engine) ScrapePage(ctx context.Context, url string, options models.ScrapingOptions) (*models.ScrapingResults, error) {
	// If depth is specified and > 0, perform multi-page crawling
	if options.Depth > 0 || options.MaxPages > 1 {
		return e.CrawlPages(ctx, url, options)
	}

	// Otherwise, perform single-page scraping (existing functionality)
	return e.scrapeSinglePageLegacy(ctx, url, options)
}

// scrapeSinglePageLegacy is the original ScrapePage method renamed for single-page scraping
func (e *Engine) scrapeSinglePageLegacy(ctx context.Context, url string, options models.ScrapingOptions) (*models.ScrapingResults, error) {
	startTime := time.Now()

	e.logger.Info("Starting page scraping",
		zap.String("url", url),
		zap.Any("options", options))

	// Create new page
	page, err := e.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Configure page
	if err := e.configurePage(page, options); err != nil {
		return nil, fmt.Errorf("failed to configure page: %w", err)
	}

	// Navigate to URL with timeout
	timeout := e.config.BrowserTimeout
	if options.Timeout > 0 {
		timeout = options.Timeout
	}

	pageLoadStart := time.Now()
	if err := page.Timeout(timeout).Navigate(url); err != nil {
		return nil, fmt.Errorf("failed to navigate to URL: %w", err)
	}

	// Wait for page to load
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("failed to wait for page load: %w", err)
	}
	pageLoadTime := time.Since(pageLoadStart)

	// Wait for additional time if specified
	if e.config.PageLoadWait > 0 {
		time.Sleep(e.config.PageLoadWait)
	}

	// Wait for specific selector if provided
	if options.WaitForSelector != "" {
		e.logger.Info("Waiting for selector", zap.String("selector", options.WaitForSelector))
		element, err := page.Timeout(timeout).Element(options.WaitForSelector)
		if err != nil {
			e.logger.Warn("Failed to find selector",
				zap.String("selector", options.WaitForSelector),
				zap.Error(err))
		} else {
			if err := element.WaitVisible(); err != nil {
				e.logger.Warn("Failed to wait for selector visibility",
					zap.String("selector", options.WaitForSelector),
					zap.Error(err))
			}
		}
	}

	// Scroll to bottom if requested
	if options.ScrollToBottom {
		if err := e.scrollToBottom(page); err != nil {
			e.logger.Warn("Failed to scroll to bottom", zap.Error(err))
		}
	}

	// Wait for JavaScript if needed
	if options.WaitForJS {
		// Wait for DOM content loaded
		if err := page.WaitDOMStable(time.Second*2, 0.1); err != nil {
			e.logger.Warn("Failed to wait for DOM stability", zap.Error(err))
		}
	}

	// Extract page metadata
	metadata, err := e.extractPageMetadata(page)
	if err != nil {
		e.logger.Warn("Failed to extract page metadata", zap.Error(err))
		metadata = &models.PageMetadata{}
	}

	// Extract module title pairs - this is the core functionality
	modulePairs, err := e.extractModuleTitlePairs(page)
	if err != nil {
		return nil, fmt.Errorf("failed to extract module title pairs: %w", err)
	}

	// Add source URL to modules for consistency
	for i := range modulePairs {
		modulePairs[i].SourceURL = url
		modulePairs[i].SourceTitle = metadata.Title
		modulePairs[i].SourceDepth = 0
	}

	// Calculate processing stats
	processingTime := time.Since(startTime)
	contentLength := e.calculateContentLength(modulePairs)

	stats := models.ProcessingStats{
		ProcessingTime:    processingTime,
		ContentLength:     contentLength,
		ModulesExtracted:  len(modulePairs),
		ImagesFound:       len(metadata.Images),
		LinksFound:        len(metadata.Links),
		JavaScriptEnabled: options.WaitForJS,
		PageLoadTime:      pageLoadTime,
		DOMContentLoaded:  pageLoadTime, // Simplified for now
		NetworkRequests:   0,            // Would need network monitoring
		ResourcesLoaded:   0,            // Would need resource monitoring
		ErrorsEncountered: 0,            // Track errors during processing
		PagesProcessed:    1,            // Single page
		CrawlDepth:        0,            // No crawling
	}

	results := &models.ScrapingResults{
		URL:             url,
		Title:           metadata.Title,
		Description:     metadata.Description,
		Language:        metadata.Language,
		ModulePairs:     modulePairs,
		Metadata:        *metadata,
		ProcessingStats: stats,
	}

	e.logger.Info("Page scraping completed successfully",
		zap.String("url", url),
		zap.Duration("processing_time", processingTime),
		zap.Int("modules_extracted", len(modulePairs)),
		zap.Int("content_length", contentLength))

	return results, nil
}

// scrollToBottom scrolls the page to the bottom to load dynamic content
func (e *Engine) scrollToBottom(page *rod.Page) error {
	// Get initial height
	initialHeight, err := page.Eval("document.body.scrollHeight")
	if err != nil {
		return err
	}

	for {
		// Scroll to bottom
		if _, err := page.Eval("window.scrollTo(0, document.body.scrollHeight)"); err != nil {
			return err
		}

		// Wait a bit for content to load
		time.Sleep(time.Millisecond * 500)

		// Check new height
		newHeight, err := page.Eval("document.body.scrollHeight")
		if err != nil {
			return err
		}

		// If height hasn't changed, we're done
		if newHeight.Value.Int() == initialHeight.Value.Int() {
			break
		}

		initialHeight = newHeight
	}

	return nil
}

// calculateContentLength calculates total content length from modules
func (e *Engine) calculateContentLength(modules []models.ModuleTitlePair) int {
	total := 0
	for _, module := range modules {
		total += len(module.Title) + len(module.Content)
		// Recursively count sub-modules
		total += e.calculateContentLength(module.SubModules)
	}
	return total
}

// extractPageMetadata extracts basic page metadata
func (e *Engine) extractPageMetadata(page *rod.Page) (*models.PageMetadata, error) {
	metadata := &models.PageMetadata{}

	// Extract title
	titleEl, err := page.Element("title")
	if err == nil {
		if title, err := titleEl.Text(); err == nil {
			metadata.Title = title
		}
	}

	// Extract description from meta tag
	if desc, err := page.Element(`meta[name="description"]`); err == nil {
		if content, err := desc.Attribute("content"); err == nil && content != nil {
			metadata.Description = *content
		}
	}

	// Extract language
	if lang, err := page.Element("html"); err == nil {
		if langAttr, err := lang.Attribute("lang"); err == nil && langAttr != nil {
			metadata.Language = *langAttr
		}
	}

	// Extract charset
	if charset, err := page.Element(`meta[charset]`); err == nil {
		if charsetAttr, err := charset.Attribute("charset"); err == nil && charsetAttr != nil {
			metadata.Charset = *charsetAttr
		}
	}

	// Extract viewport
	if viewport, err := page.Element(`meta[name="viewport"]`); err == nil {
		if content, err := viewport.Attribute("content"); err == nil && content != nil {
			metadata.Viewport = *content
		}
	}

	// Extract canonical URL
	if canonical, err := page.Element(`link[rel="canonical"]`); err == nil {
		if href, err := canonical.Attribute("href"); err == nil && href != nil {
			metadata.Canonical = *href
		}
	}

	return metadata, nil
}

// Health checks if the engine is healthy
func (e *Engine) Health(ctx context.Context) error {
	if e.browser == nil {
		return fmt.Errorf("browser not initialized")
	}

	// Try to create a simple page to test browser connectivity
	page, err := e.browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return fmt.Errorf("failed to create test page: %w", err)
	}
	defer page.Close()

	// Navigate to a simple page
	if err := page.Timeout(time.Second * 10).Navigate("data:text/html,<html><body>Health Check</body></html>"); err != nil {
		return fmt.Errorf("failed to navigate to test page: %w", err)
	}

	return nil
}
