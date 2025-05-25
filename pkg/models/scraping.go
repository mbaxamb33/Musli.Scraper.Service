// pkg/models/scraping.go
package models

import (
	"time"
)

// ScrapingResults contains the complete results of a scraping operation
type ScrapingResults struct {
	URL             string            `json:"url"`
	Title           string            `json:"title"`
	Description     string            `json:"description,omitempty"`
	Language        string            `json:"language,omitempty"`
	ModulePairs     []ModuleTitlePair `json:"module_pairs"`
	Metadata        PageMetadata      `json:"metadata"`
	ProcessingStats ProcessingStats   `json:"processing_stats"`
	CrawlResults    *CrawlResults     `json:"crawl_results,omitempty"` // Only present for multi-page crawls
}

// ModuleTitlePair represents a structured content module with its title
type ModuleTitlePair struct {
	ID                 string            `json:"id"`
	Title              string            `json:"title"`
	Content            string            `json:"content"`
	Level              int               `json:"level"`         // Hierarchy level (1-6)
	ContentType        ContentType       `json:"content_type"`  // Introduction, Process, etc.
	InformationDensity DensityLevel      `json:"density"`       // Overview, Detailed, Reference
	SemanticTags       []string          `json:"semantic_tags"` // Technical, Commercial, etc.
	RelationshipType   RelationshipType  `json:"relationship"`  // Sequential, Hierarchical, etc.
	Metadata           ModuleMetadata    `json:"metadata"`
	SubModules         []ModuleTitlePair `json:"sub_modules,omitempty"` // Nested modules
	CrossReferences    []CrossReference  `json:"cross_references,omitempty"`
	ExtractedAt        time.Time         `json:"extracted_at"`

	// New fields for crawling support
	SourceURL   string `json:"source_url,omitempty"`   // URL where this module was found
	SourceTitle string `json:"source_title,omitempty"` // Title of the source page
	SourceDepth int    `json:"source_depth,omitempty"` // Crawl depth where this was found
}

// PageMetadata contains comprehensive page-level information
type PageMetadata struct {
	Title         string      `json:"title"`
	Description   string      `json:"description,omitempty"`
	Keywords      []string    `json:"keywords,omitempty"`
	Author        string      `json:"author,omitempty"`
	PublishedDate *time.Time  `json:"published_date,omitempty"`
	LastModified  *time.Time  `json:"last_modified,omitempty"`
	ContentType   string      `json:"content_type"`
	Language      string      `json:"language,omitempty"`
	Canonical     string      `json:"canonical,omitempty"`
	Favicon       string      `json:"favicon,omitempty"`
	Images        []ImageInfo `json:"images,omitempty"`
	Links         []LinkInfo  `json:"links,omitempty"`

	// OpenGraph metadata
	OGTitle       string `json:"og_title,omitempty"`
	OGDescription string `json:"og_description,omitempty"`
	OGImage       string `json:"og_image,omitempty"`
	OGType        string `json:"og_type,omitempty"`

	// Twitter Card metadata
	TwitterCard        string `json:"twitter_card,omitempty"`
	TwitterTitle       string `json:"twitter_title,omitempty"`
	TwitterDescription string `json:"twitter_description,omitempty"`
	TwitterImage       string `json:"twitter_image,omitempty"`

	// Technical metadata
	Charset  string `json:"charset,omitempty"`
	Viewport string `json:"viewport,omitempty"`
	Robots   string `json:"robots,omitempty"`
}

// ProcessingStats contains statistics about the scraping process
type ProcessingStats struct {
	ProcessingTime    time.Duration `json:"processing_time"`
	ContentLength     int           `json:"content_length"`
	ModulesExtracted  int           `json:"modules_extracted"`
	ImagesFound       int           `json:"images_found"`
	LinksFound        int           `json:"links_found"`
	JavaScriptEnabled bool          `json:"javascript_enabled"`
	PageLoadTime      time.Duration `json:"page_load_time"`
	DOMContentLoaded  time.Duration `json:"dom_content_loaded"`
	NetworkRequests   int           `json:"network_requests"`
	ResourcesLoaded   int           `json:"resources_loaded"`
	ErrorsEncountered int           `json:"errors_encountered"`

	// New fields for crawling support
	PagesProcessed int `json:"pages_processed"` // Total pages processed
	CrawlDepth     int `json:"crawl_depth"`     // Maximum depth reached
}

// ExtractionConfig contains configuration for the extraction process
type ExtractionConfig struct {
	// Content filtering
	MinContentLength  int `json:"min_content_length"`
	MaxContentLength  int `json:"max_content_length"`
	MinWordsPerModule int `json:"min_words_per_module"`

	// Semantic analysis
	EnableSemanticTagging   bool `json:"enable_semantic_tagging"`
	EnableKeyTermExtraction bool `json:"enable_key_term_extraction"`
	EnableCrossReferences   bool `json:"enable_cross_references"`

	// Visual analysis
	EnableVisualAnalysis bool `json:"enable_visual_analysis"`
	AnalyzeImages        bool `json:"analyze_images"`
	AnalyzeLayout        bool `json:"analyze_layout"`

	// Content types to extract
	IncludeImages bool `json:"include_images"`
	IncludeLinks  bool `json:"include_links"`
	IncludeTables bool `json:"include_tables"`
	IncludeLists  bool `json:"include_lists"`
	IncludeCode   bool `json:"include_code"`

	// Hierarchy settings
	MaxNestingLevel   int  `json:"max_nesting_level"`
	PreserveHierarchy bool `json:"preserve_hierarchy"`

	// Quality settings
	SimilarityThreshold float64 `json:"similarity_threshold"`
	EnableDeduplication bool    `json:"enable_deduplication"`
}

// DefaultExtractionConfig returns a default extraction configuration
func DefaultExtractionConfig() ExtractionConfig {
	return ExtractionConfig{
		MinContentLength:        50,
		MaxContentLength:        50000,
		MinWordsPerModule:       10,
		EnableSemanticTagging:   true,
		EnableKeyTermExtraction: true,
		EnableCrossReferences:   false, // Computationally expensive
		EnableVisualAnalysis:    true,
		AnalyzeImages:           true,
		AnalyzeLayout:           true,
		IncludeImages:           true,
		IncludeLinks:            true,
		IncludeTables:           true,
		IncludeLists:            true,
		IncludeCode:             true,
		MaxNestingLevel:         6,
		PreserveHierarchy:       true,
		SimilarityThreshold:     0.8,
		EnableDeduplication:     true,
	}
}

// CrawlResults contains information about a multi-page crawl
type CrawlResults struct {
	TotalPagesFound   int           `json:"total_pages_found"`
	TotalPagesScraped int           `json:"total_pages_scraped"`
	MaxDepthReached   int           `json:"max_depth_reached"`
	CrawlDuration     time.Duration `json:"crawl_duration"`
	PagesSummary      []PageSummary `json:"pages_summary"`
}

// PageSummary contains summary information about a scraped page
type PageSummary struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	ModuleCount int       `json:"module_count"`
	Depth       int       `json:"depth"`
	ScrapedAt   time.Time `json:"scraped_at"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}
