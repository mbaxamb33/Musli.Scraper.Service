// internal/scraper/extractor.go
package scraper

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/mbaxamb3/nusli/scraper-service/pkg/models"
	"go.uber.org/zap"
)

// HeaderElement represents a header found on the page
type HeaderElement struct {
	Element *rod.Element
	Level   int // 1-6 for h1-h6
	Text    string
	XPath   string
	Index   int // Order in document
}

// extractModuleTitlePairs extracts structured content based on headers
func (e *Engine) extractModuleTitlePairs(page *rod.Page) ([]models.ModuleTitlePair, error) {
	// Step 1: Find all headers in document order
	headers, err := e.findAllHeaders(page)
	if err != nil {
		return nil, fmt.Errorf("failed to find headers: %w", err)
	}

	if len(headers) == 0 {
		e.logger.Info("No headers found, treating entire page as single module")
		return e.extractSingleModule(page)
	}

	e.logger.Info("Found headers", zap.Int("count", len(headers)))

	// Step 2: Extract content for each header section
	var modules []models.ModuleTitlePair

	for i, header := range headers {
		module, err := e.extractModuleForHeader(page, header, headers, i)
		if err != nil {
			e.logger.Warn("Failed to extract module for header",
				zap.String("header_text", header.Text),
				zap.Int("level", header.Level),
				zap.Error(err))
			continue
		}

		if module != nil {
			modules = append(modules, *module)
		}
	}

	e.logger.Info("Extracted modules", zap.Int("count", len(modules)))
	return modules, nil
}

// findAllHeaders finds all h1-h6 elements in document order
func (e *Engine) findAllHeaders(page *rod.Page) ([]HeaderElement, error) {
	// Query for all header elements
	headerElements, err := page.Elements("h1, h2, h3, h4, h5, h6")
	if err != nil {
		return nil, fmt.Errorf("failed to find header elements: %w", err)
	}

	var headers []HeaderElement

	for i, elem := range headerElements {
		// Get tag name to determine level
		tagName, err := elem.Property("tagName")
		if err != nil {
			e.logger.Warn("Failed to get tag name for header", zap.Error(err))
			continue
		}

		tagStr := strings.ToLower(tagName.String())
		levelStr := strings.TrimPrefix(tagStr, "h")
		level, err := strconv.Atoi(levelStr)
		if err != nil || level < 1 || level > 6 {
			e.logger.Warn("Invalid header level", zap.String("tag", tagStr))
			continue
		}

		// Get header text
		text, err := elem.Text()
		if err != nil {
			e.logger.Warn("Failed to get header text", zap.Error(err))
			continue
		}

		// Get XPath for reference
		xpath, err := e.getElementXPath(elem)
		if err != nil {
			e.logger.Warn("Failed to get XPath for header", zap.Error(err))
			xpath = fmt.Sprintf("//h%d[%d]", level, i+1) // Fallback
		}

		headers = append(headers, HeaderElement{
			Element: elem,
			Level:   level,
			Text:    strings.TrimSpace(text),
			XPath:   xpath,
			Index:   i,
		})
	}

	return headers, nil
}

// extractModuleForHeader extracts content for a specific header
func (e *Engine) extractModuleForHeader(page *rod.Page, header HeaderElement, allHeaders []HeaderElement, headerIndex int) (*models.ModuleTitlePair, error) {
	// Find the next boundary (next header of same or higher level)
	nextBoundaryIndex := e.findNextBoundary(header, allHeaders, headerIndex)

	// Extract content between this header and the next boundary
	content, err := e.extractContentBetweenHeaders(page, header, allHeaders, headerIndex, nextBoundaryIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content: %w", err)
	}

	// Skip empty modules
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	// Create module metadata
	metadata := e.createModuleMetadata(content, header)

	// Generate module ID
	moduleID := e.generateModuleID(header.Text, headerIndex)

	// Determine content type based on header text and content
	contentType := e.determineContentType(header.Text, content)

	// Determine information density
	density := e.determineDensityLevel(content)

	// Extract semantic tags
	semanticTags := e.extractSemanticTags(header.Text, content)

	module := &models.ModuleTitlePair{
		ID:                 moduleID,
		Title:              header.Text,
		Content:            content,
		Level:              header.Level,
		ContentType:        contentType,
		InformationDensity: density,
		SemanticTags:       semanticTags,
		RelationshipType:   models.RelationshipSequential, // Default for now
		Metadata:           metadata,
		SubModules:         []models.ModuleTitlePair{}, // Will implement nesting later
		CrossReferences:    []models.CrossReference{},  // Will implement later
		ExtractedAt:        time.Now(),
	}

	return module, nil
}

// findNextBoundary finds the next header that marks the end of current section
func (e *Engine) findNextBoundary(currentHeader HeaderElement, allHeaders []HeaderElement, currentIndex int) int {
	for i := currentIndex + 1; i < len(allHeaders); i++ {
		nextHeader := allHeaders[i]

		// Section ends when we find:
		// 1. A header of the same level
		// 2. A header of a higher level (lower number)
		if nextHeader.Level <= currentHeader.Level {
			return i
		}
	}

	// No boundary found, content goes to end of document
	return -1
}

// extractContentBetweenHeaders extracts all content between two headers
func (e *Engine) extractContentBetweenHeaders(page *rod.Page, header HeaderElement, allHeaders []HeaderElement, startIndex, endIndex int) (string, error) {
	var content strings.Builder

	// Get all elements after the header
	headerElement := header.Element

	// Strategy: Get all siblings and following elements until we reach the next boundary
	if endIndex == -1 {
		// Extract to end of document
		content.WriteString(e.extractContentToEnd(page, headerElement))
	} else {
		// Extract until next boundary header
		nextHeader := allHeaders[endIndex]
		content.WriteString(e.extractContentUntilElement(page, headerElement, nextHeader.Element))
	}

	return e.cleanContent(content.String()), nil
}

// extractContentToEnd extracts content from current element to end of document
func (e *Engine) extractContentToEnd(page *rod.Page, startElement *rod.Element) string {
	// Use JavaScript to extract content
	script := `
	function extractContentAfterElement(element) {
		let content = [];
		let current = element.nextElementSibling;
		
		while (current) {
			// Stop if we hit another header
			if (current.tagName && current.tagName.match(/^H[1-6]$/)) {
				break;
			}
			
			// Extract text content, preserving some structure
			let text = extractElementContent(current);
			if (text.trim()) {
				content.push(text);
			}
			
			current = current.nextElementSibling;
		}
		
		return content.join('\\n\\n');
	}
	
	function extractElementContent(element) {
		if (!element) return '';
		
		// Skip script and style elements
		if (element.tagName === 'SCRIPT' || element.tagName === 'STYLE') {
			return '';
		}
		
		// For text nodes and simple elements, return text content
		if (element.nodeType === Node.TEXT_NODE) {
			return element.textContent.trim();
		}
		
		// For complex elements, get inner text but preserve some structure
		let text = element.innerText || element.textContent || '';
		
		// Add extra spacing for block elements
		if (window.getComputedStyle(element).display === 'block') {
			text = text + '\\n';
		}
		
		return text.trim();
	}
	
	return extractContentAfterElement(arguments[0]);
	`

	result, err := page.Eval(script, startElement.Object)
	if err != nil {
		// Fallback: get text content directly
		parent, err := startElement.Parent()
		if err != nil {
			return ""
		}
		siblings, err := parent.Elements("*")
		if err != nil {
			return ""
		}
		var content strings.Builder
		found := false

		for _, sibling := range siblings {
			if found {
				if text, err := sibling.Text(); err == nil && strings.TrimSpace(text) != "" {
					content.WriteString(text)
					content.WriteString("\n\n")
				}
			} else if sibling.Object.ObjectID == startElement.Object.ObjectID {
				found = true
			}
		}

		return content.String()
	}

	return result.Value.String()
}

// extractContentUntilElement extracts content between start element and end element
func (e *Engine) extractContentUntilElement(page *rod.Page, startElement, endElement *rod.Element) string {
	script := `
	function extractContentBetweenElements(start, end) {
		let content = [];
		let current = start.nextElementSibling;
		
		while (current && current !== end) {
			let text = extractElementContent(current);
			if (text.trim()) {
				content.push(text);
			}
			current = current.nextElementSibling;
		}
		
		return content.join('\\n\\n');
	}
	
	function extractElementContent(element) {
		if (!element) return '';
		
		if (element.tagName === 'SCRIPT' || element.tagName === 'STYLE') {
			return '';
		}
		
		let text = element.innerText || element.textContent || '';
		return text.trim();
	}
	
	return extractContentBetweenElements(arguments[0], arguments[1]);
	`

	result, err := page.Eval(script, startElement.Object, endElement.Object)
	if err != nil {
		return ""
	}

	return result.Value.String()
}

// extractSingleModule creates a single module from the entire page when no headers are found
func (e *Engine) extractSingleModule(page *rod.Page) ([]models.ModuleTitlePair, error) {
	// Get page title as module title
	titleEl, err := page.Element("title")
	var title string
	if err != nil || titleEl == nil {
		title = "Page Content"
	} else {
		if titleText, err := titleEl.Text(); err == nil {
			title = titleText
		} else {
			title = "Page Content"
		}
	}

	// Extract body content
	body, err := page.Element("body")
	if err != nil {
		return nil, fmt.Errorf("failed to get body element: %w", err)
	}

	content, err := body.Text()
	if err != nil {
		return nil, fmt.Errorf("failed to get body text: %w", err)
	}

	content = e.cleanContent(content)
	if strings.TrimSpace(content) == "" {
		return []models.ModuleTitlePair{}, nil
	}

	metadata := e.createModuleMetadata(content, HeaderElement{Text: title, Level: 1})

	module := models.ModuleTitlePair{
		ID:                 "single-module-1",
		Title:              title,
		Content:            content,
		Level:              1,
		ContentType:        models.ContentTypeGeneral,
		InformationDensity: models.DensityDetailed,
		SemanticTags:       e.extractSemanticTags(title, content),
		RelationshipType:   models.RelationshipSequential,
		Metadata:           metadata,
		SubModules:         []models.ModuleTitlePair{},
		CrossReferences:    []models.CrossReference{},
		ExtractedAt:        time.Now(),
	}

	return []models.ModuleTitlePair{module}, nil
}

// Helper functions for module creation

func (e *Engine) generateModuleID(title string, index int) string {
	// Create a simple ID from title and index
	cleanTitle := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	// Remove special characters
	cleanTitle = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, cleanTitle)

	if len(cleanTitle) > 50 {
		cleanTitle = cleanTitle[:50]
	}

	return fmt.Sprintf("module-%d-%s", index+1, cleanTitle)
}

func (e *Engine) createModuleMetadata(content string, header HeaderElement) models.ModuleMetadata {
	wordCount := len(strings.Fields(content))
	readingTime := wordCount / 200 // Assume 200 words per minute
	if readingTime < 1 {
		readingTime = 1
	}

	return models.ModuleMetadata{
		WordCount:       wordCount,
		ReadingTime:     readingTime,
		ComplexityScore: e.calculateComplexityScore(content),
		KeyTerms:        e.extractKeyTerms(content),
		VisualElements:  []models.VisualElement{}, // TODO: Implement
		Position: models.ElementPosition{
			XPath: header.XPath,
		},
	}
}

func (e *Engine) determineContentType(title, content string) models.ContentType {
	titleLower := strings.ToLower(title)
	contentLower := strings.ToLower(content)

	switch {
	case strings.Contains(titleLower, "introduction") || strings.Contains(titleLower, "overview"):
		return models.ContentTypeIntroduction
	case strings.Contains(titleLower, "definition") || strings.Contains(titleLower, "what is"):
		return models.ContentTypeDefinition
	case strings.Contains(titleLower, "process") || strings.Contains(titleLower, "how to") || strings.Contains(titleLower, "steps"):
		return models.ContentTypeProcess
	case strings.Contains(titleLower, "example") || strings.Contains(titleLower, "sample"):
		return models.ContentTypeExample
	case strings.Contains(titleLower, "conclusion") || strings.Contains(titleLower, "summary"):
		return models.ContentTypeConclusion
	case strings.Contains(contentLower, "<li>") || strings.Count(contentLower, "\n•") > 2:
		return models.ContentTypeList
	case strings.Contains(contentLower, "<table>") || strings.Count(contentLower, "|") > 5:
		return models.ContentTypeTable
	case strings.Contains(contentLower, "<code>") || strings.Contains(contentLower, "```"):
		return models.ContentTypeCode
	default:
		return models.ContentTypeGeneral
	}
}

func (e *Engine) determineDensityLevel(content string) models.DensityLevel {
	wordCount := len(strings.Fields(content))

	switch {
	case wordCount < 100:
		return models.DensityOverview
	case wordCount < 500:
		return models.DensityDetailed
	default:
		return models.DensityReference
	}
}

func (e *Engine) extractSemanticTags(title, content string) []string {
	var tags []string

	combined := strings.ToLower(title + " " + content)

	// Technical indicators
	if strings.Contains(combined, "api") || strings.Contains(combined, "code") || strings.Contains(combined, "function") {
		tags = append(tags, "technical")
	}

	// Business indicators
	if strings.Contains(combined, "business") || strings.Contains(combined, "market") || strings.Contains(combined, "revenue") {
		tags = append(tags, "business")
	}

	// Educational indicators
	if strings.Contains(combined, "learn") || strings.Contains(combined, "tutorial") || strings.Contains(combined, "guide") {
		tags = append(tags, "educational")
	}

	return tags
}

func (e *Engine) calculateComplexityScore(content string) float64 {
	// Simple complexity based on sentence length and vocabulary
	sentences := strings.Split(content, ".")
	if len(sentences) == 0 {
		return 0.0
	}

	totalWords := len(strings.Fields(content))
	avgSentenceLength := float64(totalWords) / float64(len(sentences))

	// Normalize to 0-1 scale
	complexity := avgSentenceLength / 30.0 // 30 words = max complexity
	if complexity > 1.0 {
		complexity = 1.0
	}

	return complexity
}

func (e *Engine) extractKeyTerms(content string) []string {
	// Simple keyword extraction - look for capitalized words and repeated terms
	words := strings.Fields(content)
	wordCount := make(map[string]int)

	for _, word := range words {
		word = strings.Trim(word, ".,!?:;\"'")
		if len(word) > 3 && (strings.Title(word) == word || strings.ToUpper(word) == word) {
			wordCount[word]++
		}
	}

	var keyTerms []string
	for word, count := range wordCount {
		if count > 1 { // Appears more than once
			keyTerms = append(keyTerms, word)
		}
		if len(keyTerms) >= 10 { // Limit to top 10
			break
		}
	}

	return keyTerms
}

func (e *Engine) cleanContent(content string) string {
	// Remove excessive whitespace
	content = strings.ReplaceAll(content, "\t", " ")
	lines := strings.Split(content, "\n")

	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

func (e *Engine) getElementXPath(elem *rod.Element) (string, error) {
	// Generate XPath for element - simplified version
	result, err := elem.Eval(`
		function getXPath(element) {
			if (element.id) {
				return '//*[@id="' + element.id + '"]';
			}
			
			let path = '';
			while (element && element.nodeType === Node.ELEMENT_NODE) {
				let tagName = element.nodeName.toLowerCase();
				let siblings = Array.from(element.parentNode ? element.parentNode.children : []);
				let sameTagSiblings = siblings.filter(sibling => sibling.nodeName.toLowerCase() === tagName);
				
				if (sameTagSiblings.length > 1) {
					let index = sameTagSiblings.indexOf(element) + 1;
					path = '/' + tagName + '[' + index + ']' + path;
				} else {
					path = '/' + tagName + path;
				}
				
				element = element.parentNode;
				if (element && element.nodeName.toLowerCase() === 'html') {
					break;
				}
			}
			
			return path || '/';
		}
		
		return getXPath(this);
	`)

	if err != nil {
		return "", err
	}

	return result.Value.String(), nil
}
