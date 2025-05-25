// internal/scraper/crawler.go
package scraper

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
	"go.uber.org/zap"
)

// CrawlJob represents a single page to be crawled
type CrawlJob struct {
	URL   string
	Depth int
}

// CrawlState manages the state of a multi-page crawl
type CrawlState struct {
	visitedURLs  map[string]bool
	pendingJobs  []CrawlJob
	results      []models.ModuleTitlePair
	pagesSummary []models.PageSummary
	mu           sync.RWMutex
	baseDomain   string
	maxDepth     int
	maxPages     int
	pagesScraped int
	startTime    time.Time
}

// NewCrawlState creates a new crawl state
func NewCrawlState(rootURL string, maxDepth, maxPages int) (*CrawlState, error) {
	parsedURL, err := url.Parse(rootURL)
	if err != nil {
		return nil, fmt.Errorf("invalid root URL: %w", err)
	}

	return &CrawlState{
		visitedURLs:  make(map[string]bool),
		pendingJobs:  []CrawlJob{{URL: rootURL, Depth: 0}},
		results:      []models.ModuleTitlePair{},
		pagesSummary: []models.PageSummary{},
		baseDomain:   parsedURL.Host,
		maxDepth:     maxDepth,
		maxPages:     maxPages,
		startTime:    time.Now(),
	}, nil
}

// CrawlPages performs multi-page crawling with depth control
func (e *Engine) CrawlPages(ctx context.Context, rootURL string, options models.ScrapingOptions) (*models.ScrapingResults, error) {
	startTime := time.Now()

	e.logger.Info("Starting multi-page crawl",
		zap.String("root_url", rootURL),
		zap.Int("depth", options.Depth),
		zap.Int("max_pages", options.MaxPages))

	// Set default max pages if not specified
	maxPages := options.MaxPages
	if maxPages <= 0 {
		maxPages = 10 // Default limit
	}

	// Initialize crawl state
	crawlState, err := NewCrawlState(rootURL, options.Depth, maxPages)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize crawl state: %w", err)
	}

	// Process pages level by level
	for currentDepth := 0; currentDepth <= options.Depth; currentDepth++ {
		if err := e.processDepthLevel(ctx, crawlState, currentDepth, options); err != nil {
			e.logger.Error("Error processing depth level",
				zap.Int("depth", currentDepth),
				zap.Error(err))
			break
		}

		// Check if we've reached max pages
		if crawlState.pagesScraped >= maxPages {
			e.logger.Info("Reached maximum page limit", zap.Int("pages_scraped", crawlState.pagesScraped))
			break
		}
	}

	// Create results
	crawlDuration := time.Since(startTime)
	results := &models.ScrapingResults{
		URL:         rootURL,
		ModulePairs: crawlState.results,
		ProcessingStats: models.ProcessingStats{
			ProcessingTime:   crawlDuration,
			ModulesExtracted: len(crawlState.results),
			PagesProcessed:   crawlState.pagesScraped,
			CrawlDepth:       options.Depth,
		},
		CrawlResults: &models.CrawlResults{
			TotalPagesFound:   len(crawlState.visitedURLs),
			TotalPagesScraped: crawlState.pagesScraped,
			MaxDepthReached:   options.Depth,
			CrawlDuration:     crawlDuration,
			PagesSummary:      crawlState.pagesSummary,
		},
	}

	// Get metadata from root page if available
	if len(crawlState.pagesSummary) > 0 {
		results.Title = crawlState.pagesSummary[0].Title
	}

	e.logger.Info("Multi-page crawl completed",
		zap.String("root_url", rootURL),
		zap.Int("pages_scraped", crawlState.pagesScraped),
		zap.Int("modules_extracted", len(crawlState.results)),
		zap.Duration("duration", crawlDuration))

	return results, nil
}

// processDepthLevel processes all pages at a specific depth level
func (e *Engine) processDepthLevel(ctx context.Context, crawlState *CrawlState, targetDepth int, options models.ScrapingOptions) error {
	crawlState.mu.Lock()
	currentJobs := []CrawlJob{}
	remainingJobs := []CrawlJob{}

	// Separate jobs by depth
	for _, job := range crawlState.pendingJobs {
		if job.Depth == targetDepth {
			currentJobs = append(currentJobs, job)
		} else {
			remainingJobs = append(remainingJobs, job)
		}
	}
	crawlState.pendingJobs = remainingJobs
	crawlState.mu.Unlock()

	// Process current depth jobs
	for _, job := range currentJobs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check page limits
		if crawlState.pagesScraped >= crawlState.maxPages {
			break
		}

		// Skip if already visited
		crawlState.mu.RLock()
		visited := crawlState.visitedURLs[job.URL]
		crawlState.mu.RUnlock()

		if visited {
			continue
		}

		// Mark as visited
		crawlState.mu.Lock()
		crawlState.visitedURLs[job.URL] = true
		crawlState.mu.Unlock()

		// Scrape the page
		pageResults, err := e.scrapeSinglePage(ctx, job.URL, job.Depth, options)
		if err != nil {
			e.logger.Warn("Failed to scrape page",
				zap.String("url", job.URL),
				zap.Int("depth", job.Depth),
				zap.Error(err))

			// Add to summary with error
			crawlState.mu.Lock()
			crawlState.pagesSummary = append(crawlState.pagesSummary, models.PageSummary{
				URL:       job.URL,
				Depth:     job.Depth,
				ScrapedAt: time.Now(),
				Success:   false,
				Error:     err.Error(),
			})
			crawlState.mu.Unlock()
			continue
		}

		// Add results
		crawlState.mu.Lock()
		crawlState.results = append(crawlState.results, pageResults.modules...)
		crawlState.pagesSummary = append(crawlState.pagesSummary, pageResults.summary)
		crawlState.pagesScraped++
		crawlState.mu.Unlock()

		// Find links for next depth level
		if job.Depth < options.Depth {
			newJobs := e.extractLinksForNextDepth(pageResults.links, job.Depth+1, options, crawlState.baseDomain)

			crawlState.mu.Lock()
			crawlState.pendingJobs = append(crawlState.pendingJobs, newJobs...)
			crawlState.mu.Unlock()
		}

		// Add delay between requests
		if e.config.RequestDelay > 0 {
			time.Sleep(e.config.RequestDelay)
		}
	}

	return nil
}

// PageScrapingResult holds results from scraping a single page
type PageScrapingResult struct {
	modules []models.ModuleTitlePair
	links   []string
	summary models.PageSummary
}

// scrapeSinglePage scrapes a single page and returns modules and links
func (e *Engine) scrapeSinglePage(ctx context.Context, pageURL string, depth int, options models.ScrapingOptions) (*PageScrapingResult, error) {
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

	// Navigate to URL
	timeout := e.config.BrowserTimeout
	if options.Timeout > 0 {
		timeout = options.Timeout
	}

	if err := page.Timeout(timeout).Navigate(pageURL); err != nil {
		return nil, fmt.Errorf("failed to navigate to URL: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("failed to wait for page load: %w", err)
	}

	// Extract page metadata
	metadata, err := e.extractPageMetadata(page)
	if err != nil {
		e.logger.Warn("Failed to extract page metadata", zap.Error(err))
		metadata = &models.PageMetadata{}
	}

	// Extract module title pairs
	modules, err := e.extractModuleTitlePairs(page)
	if err != nil {
		return nil, fmt.Errorf("failed to extract modules: %w", err)
	}

	// Add source information to modules
	for i := range modules {
		modules[i].SourceURL = pageURL
		modules[i].SourceTitle = metadata.Title
		modules[i].SourceDepth = depth
	}

	// Extract links for next depth
	links, err := e.extractAllLinks(page)
	if err != nil {
		e.logger.Warn("Failed to extract links", zap.Error(err))
		links = []string{}
	}

	// Create summary
	summary := models.PageSummary{
		URL:         pageURL,
		Title:       metadata.Title,
		ModuleCount: len(modules),
		Depth:       depth,
		ScrapedAt:   time.Now(),
		Success:     true,
	}

	return &PageScrapingResult{
		modules: modules,
		links:   links,
		summary: summary,
	}, nil
}

// extractAllLinks extracts all links from a page
func (e *Engine) extractAllLinks(page *rod.Page) ([]string, error) {
	elements, err := page.Elements("a[href]")
	if err != nil {
		return nil, err
	}

	// Get current page URL for resolving relative links
	pageInfo, err := page.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get page info: %w", err)
	}

	baseURL, err := url.Parse(pageInfo.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	var links []string
	for _, elem := range elements {
		href, err := elem.Attribute("href")
		if err != nil || href == nil {
			continue
		}

		// Resolve relative URLs
		resolvedURL, err := baseURL.Parse(*href)
		if err != nil {
			continue
		}

		links = append(links, resolvedURL.String())
	}

	return links, nil
}

// extractLinksForNextDepth filters and prepares links for the next crawl depth
func (e *Engine) extractLinksForNextDepth(links []string, nextDepth int, options models.ScrapingOptions, baseDomain string) []CrawlJob {
	var jobs []CrawlJob
	seen := make(map[string]bool)

	for _, link := range links {
		// Skip if already seen in this batch
		if seen[link] {
			continue
		}
		seen[link] = true

		// Parse URL
		parsedURL, err := url.Parse(link)
		if err != nil {
			continue
		}

		// Check domain restriction
		if options.SameDomainOnly && parsedURL.Host != baseDomain {
			continue
		}

		// Check exclude patterns
		if e.matchesPatterns(link, options.ExcludePatterns) {
			continue
		}

		// Check include patterns (if specified)
		if len(options.IncludePatterns) > 0 && !e.matchesPatterns(link, options.IncludePatterns) {
			continue
		}

		// Skip common non-content URLs
		if e.shouldSkipURL(link) {
			continue
		}

		jobs = append(jobs, CrawlJob{
			URL:   link,
			Depth: nextDepth,
		})
	}

	return jobs
}

// matchesPatterns checks if a URL matches any of the given patterns
func (e *Engine) matchesPatterns(url string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, url)
		if err != nil {
			e.logger.Warn("Invalid regex pattern", zap.String("pattern", pattern))
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// shouldSkipURL determines if a URL should be skipped
func (e *Engine) shouldSkipURL(url string) bool {
	skipPatterns := []string{
		`\.(pdf|doc|docx|xls|xlsx|ppt|pptx|zip|rar|tar|gz)$`,
		`\.(jpg|jpeg|png|gif|bmp|svg|ico)$`,
		`\.(mp3|mp4|avi|mov|wmv|flv)$`,
		`^mailto:`,
		`^tel:`,
		`^javascript:`,
		`#`,
	}

	url = strings.ToLower(url)
	for _, pattern := range skipPatterns {
		matched, _ := regexp.MatchString(pattern, url)
		if matched {
			return true
		}
	}
	return false
}

// configurePage configures a page with the given options
func (e *Engine) configurePage(page *rod.Page, options models.ScrapingOptions) error {
	// Set viewport
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  int(e.config.BrowserViewportWidth),
		Height: int(e.config.BrowserViewportHeight),
	}); err != nil {
		e.logger.Warn("Failed to set viewport", zap.Error(err))
	}

	// Set user agent
	userAgent := options.UserAgent
	if userAgent == "" {
		userAgent = e.config.BrowserUserAgent
	}

	if userAgent != "" {
		if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
			UserAgent: userAgent,
		}); err != nil {
			e.logger.Warn("Failed to set user agent", zap.Error(err))
		}
	}

	return nil
}
