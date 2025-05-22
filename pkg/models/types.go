// pkg/models/types.go
package models

// JobStatus represents the status of a scraping job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCanceled   JobStatus = "canceled"
)

// ContentType represents the type of content in a module
type ContentType string

const (
	ContentTypeIntroduction ContentType = "introduction"
	ContentTypeDefinition   ContentType = "definition"
	ContentTypeProcess      ContentType = "process"
	ContentTypeExample      ContentType = "example"
	ContentTypeConclusion   ContentType = "conclusion"
	ContentTypeList         ContentType = "list"
	ContentTypeTable        ContentType = "table"
	ContentTypeQuote        ContentType = "quote"
	ContentTypeCode         ContentType = "code"
	ContentTypeGeneral      ContentType = "general"
)

// DensityLevel represents information density
type DensityLevel string

const (
	DensityOverview  DensityLevel = "overview"
	DensityDetailed  DensityLevel = "detailed"
	DensityReference DensityLevel = "reference"
	DensityExample   DensityLevel = "example"
)

// RelationshipType represents how modules relate to each other
type RelationshipType string

const (
	RelationshipSequential    RelationshipType = "sequential"
	RelationshipHierarchical  RelationshipType = "hierarchical"
	RelationshipComparative   RelationshipType = "comparative"
	RelationshipSupplementary RelationshipType = "supplementary"
)

// ElementPosition represents the position of an element on the page
type ElementPosition struct {
	XPath    string  `json:"xpath"`
	CSSPath  string  `json:"css_path"`
	TopPx    float64 `json:"top_px"`
	LeftPx   float64 `json:"left_px"`
	WidthPx  float64 `json:"width_px"`
	HeightPx float64 `json:"height_px"`
}

// VisualElement represents a visual element found on the page
type VisualElement struct {
	Type        string `json:"type"` // image, video, chart, etc.
	Description string `json:"description"`
	URL         string `json:"url,omitempty"`
	Alt         string `json:"alt,omitempty"`
	Caption     string `json:"caption,omitempty"`
}

// CrossReference represents a reference between modules
type CrossReference struct {
	TargetModuleID string  `json:"target_module_id"`
	RelationType   string  `json:"relation_type"` // "references", "elaborates", "contradicts"
	Confidence     float64 `json:"confidence"`
}

// ModuleMetadata contains metadata about a content module
type ModuleMetadata struct {
	WordCount       int             `json:"word_count"`
	ReadingTime     int             `json:"reading_time_minutes"`
	ComplexityScore float64         `json:"complexity_score"`
	KeyTerms        []string        `json:"key_terms"`
	VisualElements  []VisualElement `json:"visual_elements"`
	Position        ElementPosition `json:"position"`
}

// ImageInfo represents information about an image
type ImageInfo struct {
	URL     string `json:"url"`
	Alt     string `json:"alt,omitempty"`
	Caption string `json:"caption,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
}

// LinkInfo represents information about a link
type LinkInfo struct {
	URL          string `json:"url"`
	Text         string `json:"text"`
	Title        string `json:"title,omitempty"`
	Relationship string `json:"relationship,omitempty"` // internal, external, download
}
