// internal/scraper/extractor.go
package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/mbaxamb3/nusli/scraper-service/internal/config"
	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
	"go.uber.org/zap"
)

// ContentExtractor handles intelligent content extraction
type ContentExtractor struct {
	config *config.Config
	logger *zap.Logger
}

// NewContentExtractor creates a new content extractor
func NewContentExtractor(cfg *config.Config, logger *zap.Logger) *ContentExtractor {
	return &ContentExtractor{
		config: cfg,
		logger: logger,
	}
}

// HeaderInfo represents extracted header information
type HeaderInfo struct {
	Element   *rod.Element
	Text      string
	Tag       string
	Level     int
	XPath     string
	CSSPath   string
	Position  models.ElementPosition
	FontSize  float64
	IsVisible bool
}

// ContentSection represents a section of content
type ContentSection struct {
	Header  *HeaderInfo
	Content []string
	Level   int
	Start   int
	End     int
}

// ExtractModules performs intelligent module extraction using the Header + Associated Content approach
func (e *ContentExtractor) ExtractModules(page *rod.Page, url string) ([]models.ModuleTitlePair, error) {
	e.logger.Info("Starting intelligent module extraction", zap.String("url", url))

	// Step 1: Extract all headers with semantic analysis
	headers, err := e.extractHeaders(page)
	if err != nil {
		return nil, fmt.Errorf("failed to extract headers: %w", err)
	}

	e.logger.Debug("Extracted headers", zap.Int("count", len(headers)))

	// Step 2: Analyze header hierarchy and relationships
	hierarchy := e.analyzeHeaderHierarchy(headers)

	// Step 3: Define section boundaries
	sections := e.defineSectionBoundaries(page, hierarchy)

	// Step 4: Extract content for each section
	modules := make([]models.ModuleTitlePair, 0, len(sections))

	for i, section := range sections {
		module, err := e.createModuleFromSection(page, section, i)
		if err != nil {
			e.logger.Warn("Failed to create module from section",
				zap.Int("section_index", i),
				zap.Error(err))
			continue
		}

		// Apply content quality filters
		if e.isValidModule(module) {
			modules = append(modules, *module)
		}
	}

	// Step 5: Enhance modules with semantic analysis
	e.enhanceModulesWithSemantics(modules)

	e.logger.Info("Module extraction completed",
		zap.Int("total_modules", len(modules)),
		zap.String("url", url))

	return modules, nil
}

// extractHeaders extracts all headers with comprehensive analysis
func (e *ContentExtractor) extractHeaders(page *rod.Page) ([]*HeaderInfo, error) {
	// Extract all heading elements
	headerElements, err := page.Elements("h1, h2, h3, h4, h5, h6")
	if err != nil {
		return nil, fmt.Errorf("failed to find header elements: %w", err)
	}

	var headers []*HeaderInfo

	for _, element := range headerElements {
		header, err := e.analyzeHeader(element)
		if err != nil {
			e.logger.Debug("Failed to analyze header", zap.Error(err))
			continue
		}

		if header.IsVisible && header.Text != "" {
			headers = append(headers, header)
		}
	}

	return headers, nil
}

// analyzeHeader performs comprehensive header analysis
func (e *ContentExtractor) analyzeHeader(element *rod.Element) (*HeaderInfo, error) {
	// Get basic info
	text, err := element.Text()
	if err != nil {
		return nil, fmt.Errorf("failed to get header text: %w", err)
	}

	tagName, err := element.Eval("this.tagName.toLowerCase()")
	if err != nil {
		return nil, fmt.Errorf("failed to get tag name: %w", err)
	}

	// Extract level from tag name (h1=1, h2=2, etc.)
	level := 1
	if len(tagName.Str()) == 2 && tagName.Str()[0] == 'h' {
		if l, err := strconv.Atoi(string(tagName.Str()[1])); err == nil {
			level = l
		}
	}

	// Get position and styling info
	position, err := e.getElementPosition(element)
	if err != nil {
		e.logger.Debug("Failed to get element position", zap.Error(err))
		position = models.ElementPosition{}
	}

	// Get font size for visual hierarchy analysis
	fontSize := e.getElementFontSize(element)

	// Check visibility
	isVisible, err := element.Visible()
	if err != nil {
		isVisible = true // Assume visible if can't determine
	}

	// Generate XPath and CSS path
	xpath := e.generateXPath(element)
	cssPath := e.generateCSSPath(element)

	return &HeaderInfo{
		Element:   element,
		Text:      strings.TrimSpace(text),
		Tag:       tagName.Str(),
		Level:     level,
		XPath:     xpath,
		CSSPath:   cssPath,
		Position:  position,
		FontSize:  fontSize,
		IsVisible: isVisible,
	}, nil
}

// analyzeHeaderHierarchy analyzes relationships between headers
func (e *ContentExtractor) analyzeHeaderHierarchy(headers []*HeaderInfo) []*HeaderInfo {
	// Sort headers by their position on the page
	// This is a simplified version - in a real implementation you'd sort by document order

	// Analyze visual hierarchy (font size vs semantic hierarchy)
	for i, header := range headers {
		// Detect visual hierarchy discrepancies
		if i > 0 {
			prevHeader := headers[i-1]

			// If font size is larger but semantic level is lower, adjust importance
			if header.FontSize > prevHeader.FontSize && header.Level > prevHeader.Level {
				e.logger.Debug("Visual hierarchy conflict detected",
					zap.String("current_header", header.Text),
					zap.Float64("current_font_size", header.FontSize),
					zap.Int("current_level", header.Level))
			}
		}
	}

	return headers
}

// defineSectionBoundaries defines content sections based on header hierarchy
func (e *ContentExtractor) defineSectionBoundaries(page *rod.Page, headers []*HeaderInfo) []*ContentSection {
	if len(headers) == 0 {
		return []*ContentSection{}
	}

	sections := make([]*ContentSection, 0, len(headers))

	for i, header := range headers {
		section := &ContentSection{
			Header: header,
			Level:  header.Level,
			Start:  i,
		}

		// Find the end boundary for this section
		section.End = e.findSectionEnd(headers, i)

		// Extract content between this header and the next section
		content, err := e.extractSectionContent(page, header, section.End, headers)
		if err != nil {
			e.logger.Warn("Failed to extract section content",
				zap.String("header", header.Text),
				zap.Error(err))
			continue
		}

		section.Content = content
		sections = append(sections, section)
	}

	return sections
}

// findSectionEnd finds where a section ends based on header hierarchy
func (e *ContentExtractor) findSectionEnd(headers []*HeaderInfo, currentIndex int) int {
	if currentIndex >= len(headers)-1 {
		return len(headers)
	}

	currentLevel := headers[currentIndex].Level

	// Find next header of equal or higher rank
	for i := currentIndex + 1; i < len(headers); i++ {
		if headers[i].Level <= currentLevel {
			return i
		}
	}

	return len(headers)
}

// extractSectionContent extracts content between headers
func (e *ContentExtractor) extractSectionContent(page *rod.Page, header *HeaderInfo, endIndex int, allHeaders []*HeaderInfo) ([]string, error) {
	var content []string

	// This is a simplified approach - in a real implementation you'd:
	// 1. Find all elements between the current header and the next header
	// 2. Extract text content from paragraphs, lists, etc.
	// 3. Preserve structure and formatting

	// For now, we'll extract the next few paragraphs after the header
	nextElements, err := page.Elements("p, div, ul, ol, blockquote")
	if err != nil {
		return content, err
	}

	// Simple heuristic: take elements that appear after this header
	for _, element := range nextElements {
		text, err := element.Text()
		if err != nil {
			continue
		}

		text = strings.TrimSpace(text)
		if len(text) >= e.config.MinContentLength {
			content = append(content, text)
		}

		// Limit content per section
		if len(content) >= 5 {
			break
		}
	}

	return content, nil
}

// createModuleFromSection creates a module from a content section
func (e *ContentExtractor) createModuleFromSection(page *rod.Page, section *ContentSection, index int) (*models.ModuleTitlePair, error) {
	// Generate unique ID
	moduleID := fmt.Sprintf("module_%d_%d", index, time.Now().Unix())

	// Combine content
	combinedContent := strings.Join(section.Content, "\n\n")

	// Determine content type based on header text and content
	contentType := e.classifyContentType(section.Header.Text, combinedContent)

	// Determine information density
	density := e.analyzeDensity(combinedContent)

	// Extract semantic tags
	semanticTags := e.extractSemanticTags(section.Header.Text, combinedContent)

	// Create module metadata
	metadata := models.ModuleMetadata{
		WordCount:       len(strings.Fields(combinedContent)),
		ReadingTime:     e.calculateReadingTime(combinedContent),
		ComplexityScore: e.calculateComplexityScore(combinedContent),
		KeyTerms:        e.extractKeyTerms(combinedContent),
		Position:        section.Header.Position,
	}

	module := &models.ModuleTitlePair{
		ID:                 moduleID,
		Title:              section.Header.Text,
		Content:            combinedContent,
		Level:              section.Level,
		ContentType:        contentType,
		InformationDensity: density,
		SemanticTags:       semanticTags,
		RelationshipType:   models.RelationshipHierarchical, // Default to hierarchical
		Metadata:           metadata,
		ExtractedAt:        time.Now(),
	}

	return module, nil
}

// Helper methods for content analysis

func (e *ContentExtractor) classifyContentType(title, content string) models.ContentType {
	title = strings.ToLower(title)
	content = strings.ToLower(content)

	// Simple classification based on keywords
	if strings.Contains(title, "introduction") || strings.Contains(title, "overview") {
		return models.ContentTypeIntroduction
	}
	if strings.Contains(title, "process") || strings.Contains(title, "step") {
		return models.ContentTypeProcess
	}
	if strings.Contains(title, "example") || strings.Contains(content, "for example") {
		return models.ContentTypeExample
	}
	if strings.Contains(title, "conclusion") || strings.Contains(title, "summary") {
		return models.ContentTypeConclusion
	}

	return models.ContentTypeGeneral
}

func (e *ContentExtractor) analyzeDensity(content string) models.DensityLevel {
	wordCount := len(strings.Fields(content))

	if wordCount < 50 {
		return models.DensityOverview
	} else if wordCount < 200 {
		return models.DensityDetailed
	} else {
		return models.DensityReference
	}
}

func (e *ContentExtractor) extractSemanticTags(title, content string) []string {
	var tags []string

	text := strings.ToLower(title + " " + content)

	// Technical terms
	if regexp.MustCompile(`\b(api|code|programming|software|technical|algorithm)\b`).MatchString(text) {
		tags = append(tags, "technical")
	}

	// Business terms
	if regexp.MustCompile(`\b(business|commercial|market|sales|revenue)\b`).MatchString(text) {
		tags = append(tags, "business")
	}

	// Educational terms
	if regexp.MustCompile(`\b(learn|education|tutorial|guide|instruction)\b`).MatchString(text) {
		tags = append(tags, "educational")
	}

	return tags
}

func (e *ContentExtractor) calculateReadingTime(content string) int {
	wordCount := len(strings.Fields(content))
	// Average reading speed: 200 words per minute
	return (wordCount + 199) / 200
}

func (e *ContentExtractor) calculateComplexityScore(content string) float64 {
	// Simple complexity calculation based on sentence length and word complexity
	sentences := strings.Split(content, ".")
	totalWords := len(strings.Fields(content))

	if len(sentences) == 0 {
		return 0.0
	}

	avgWordsPerSentence := float64(totalWords) / float64(len(sentences))

	// Normalize to 0-1 scale
	complexity := avgWordsPerSentence / 30.0
	if complexity > 1.0 {
		complexity = 1.0
	}

	return complexity
}

func (e *ContentExtractor) extractKeyTerms(content string) []string {
	// Simple key term extraction using common patterns
	words := strings.Fields(strings.ToLower(content))
	termFreq := make(map[string]int)

	for _, word := range words {
		// Clean word and check if it's significant
		word = regexp.MustCompile(`[^\w]`).ReplaceAllString(word, "")
		if len(word) > 4 && !e.isStopWord(word) {
			termFreq[word]++
		}
	}

	// Return top terms
	var keyTerms []string
	for term, freq := range termFreq {
		if freq > 1 {
			keyTerms = append(keyTerms, term)
		}
		if len(keyTerms) >= 5 {
			break
		}
	}

	return keyTerms
}

func (e *ContentExtractor) isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true, "not": true,
		"you": true, "all": true, "can": true, "had": true, "her": true, "was": true,
		"one": true, "our": true, "out": true, "day": true, "get": true, "has": true,
		"him": true, "his": true, "how": true, "man": true, "new": true, "now": true,
		"old": true, "see": true, "two": true, "way": true, "who": true, "boy": true,
		"did": true, "its": true, "let": true, "put": true, "say": true, "she": true,
		"too": true, "use": true,
	}
	return stopWords[word]
}

// Helper methods for DOM analysis

func (e *ContentExtractor) getElementPosition(element *rod.Element) (models.ElementPosition, error) {
	// Get element position and dimensions
	box, err := element.Box()
	if err != nil {
		return models.ElementPosition{}, err
	}

	return models.ElementPosition{
		TopPx:    box.Top,
		LeftPx:   box.Left,
		WidthPx:  box.Width,
		HeightPx: box.Height,
	}, nil
}

func (e *ContentExtractor) getElementFontSize(element *rod.Element) float64 {
	// Get computed font size
	if fontSize, err := element.Eval("getComputedStyle(this).fontSize"); err == nil {
		if sizeStr := fontSize.Str(); sizeStr != "" {
			// Remove 'px' and convert to float
			sizeStr = strings.TrimSuffix(sizeStr, "px")
			if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
				return size
			}
		}
	}
	return 16.0 // Default font size
}

func (e *ContentExtractor) generateXPath(element *rod.Element) string {
	// Simplified XPath generation
	return fmt.Sprintf("//h%d[contains(text(),'%s')]", 1, "header")
}

func (e *ContentExtractor) generateCSSPath(element *rod.Element) string {
	// Simplified CSS path generation
	return "h1, h2, h3, h4, h5, h6"
}

// isValidModule checks if a module meets quality criteria
func (e *ContentExtractor) isValidModule(module *models.ModuleTitlePair) bool {
	// Check minimum content length
	if len(module.Content) < e.config.MinContentLength {
		return false
	}

	// Check maximum content length
	if len(module.Content) > e.config.MaxContentLength {
		return false
	}

	// Check minimum word count
	if module.Metadata.WordCount < e.config.MinWordsPerModule {
		return false
	}

	return true
}

// enhanceModulesWithSemantics adds semantic analysis to modules
func (e *ContentExtractor) enhanceModulesWithSemantics(modules []models.ModuleTitlePair) {
	if !e.config.EnableSemanticTagging {
		return
	}

	// Add cross-references between modules
	if e.config.EnableCrossReferences {
		for i := range modules {
			modules[i].CrossReferences = e.findCrossReferences(modules[i], modules)
		}
	}
}

// findCrossReferences finds relationships between modules
func (e *ContentExtractor) findCrossReferences(currentModule models.ModuleTitlePair, allModules []models.ModuleTitlePair) []models.CrossReference {
	var references []models.CrossReference

	for _, module := range allModules {
		if module.ID == currentModule.ID {
			continue
		}

		// Simple similarity check based on common keywords
		similarity := e.calculateSimilarity(currentModule.Content, module.Content)
		if similarity > e.config.ContentSimilarityThreshold {
			references = append(references, models.CrossReference{
				TargetModuleID: module.ID,
				RelationType:   "references",
				Confidence:     similarity,
			})
		}
	}

	return references
}

// calculateSimilarity calculates content similarity between two texts
func (e *ContentExtractor) calculateSimilarity(text1, text2 string) float64 {
	// Simple similarity calculation using common words
	words1 := strings.Fields(strings.ToLower(text1))
	words2 := strings.Fields(strings.ToLower(text2))

	set1 := make(map[string]bool)
	for _, word := range words1 {
		if len(word) > 4 && !e.isStopWord(word) {
			set1[word] = true
		}
	}

	set2 := make(map[string]bool)
	for _, word := range words2 {
		if len(word) > 4 && !e.isStopWord(word) {
			set2[word] = true
		}
	}

	// Calculate Jaccard similarity
	intersection := 0
	for word := range set1 {
		if set2[word] {
			intersection++
		}
	}

	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}
