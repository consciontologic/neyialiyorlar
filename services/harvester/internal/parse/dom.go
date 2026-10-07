package parse

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// DOM provides heuristic HTML extraction anchored on stable labels.
// Rationale (mandate 22): Turkish report labels are far more stable than
// generated class names. If DOM structure drifts, extraction fails gracefully
// with `approx` flag, never crashes.
type DOM struct {
	// Anchors: map of field name -> label text
	// Example: {"price": "Fon Toplam Değeri", "date": "Raporlama Tarihi"}
	Anchors map[string]string

	// CaseLang: language for case folding when matching labels (default: Turkish)
	CaseLang language.Tag

	// doc is the parsed HTML document
	doc *html.Node
}

// NewDOM creates a DOM extractor with Turkish-aware label anchoring.
func NewDOM(anchors map[string]string) *DOM {
	return &DOM{
		Anchors:  anchors,
		CaseLang: language.Turkish,
	}
}

// Parse extracts field values from HTML using label anchors.
// Returns a ParseResult with the extracted fields or an error reason.
func (d *DOM) Parse(htmlBytes []byte) model.ParseResult {
	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(htmlBytes)))
	if err != nil {
		return model.ParseResult{
			Confidence: model.Stale,
			Error:      err,
			Reason:     "dom_parse_error",
		}
	}

	d.doc = doc

	// Extract fields from anchors
	data := make(map[string]interface{})
	confidence := model.Fresh
	var missingFields []string

	for fieldName, label := range d.Anchors {
		value, found := d.findByLabel(label)
		if found && value != "" {
			data[fieldName] = value
		} else {
			missingFields = append(missingFields, fieldName)
			// Mark as approx if any field is missing
			confidence = model.Approx
		}
	}

	if len(missingFields) > 0 {
		return model.ParseResult{
			Data:       data,
			Confidence: confidence,
			Error:      fmt.Errorf("missing fields: %v", missingFields),
			Reason:     "dom_field_not_found",
		}
	}

	return model.ParseResult{
		Data:       data,
		Confidence: confidence,
	}
}

// findByLabel locates a value by its label, tolerating DOM drift.
// Returns the extracted value and a found flag.
func (d *DOM) findByLabel(label string) (string, bool) {
	// Walk the DOM looking for text that matches the label
	var walker func(*html.Node) string
	walker = func(n *html.Node) string {
		if n == nil {
			return ""
		}

		switch n.Type {
		case html.TextNode:
			// Check if this text node matches the label
			if d.textMatchesLabel(n.Data, label) {
				// Found the label; extract the value from a nearby cell
				if value := d.nearestValueCell(n); value != "" {
					return value
				}
			}
		}

		// Walk children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := walker(c); result != "" {
				return result
			}
		}

		return ""
	}

	value := walker(d.doc)
	return value, value != ""
}

// textMatchesLabel checks if text content matches a label (case-insensitive, Turkish-aware).
func (d *DOM) textMatchesLabel(text, label string) bool {
	// Trim and fold both sides using Turkish casing
	caser := cases.Lower(d.CaseLang)
	textLower := strings.TrimSpace(caser.String(text))
	labelLower := strings.TrimSpace(caser.String(label))

	// Exact match or substring match (label could be part of a longer text)
	return textLower == labelLower || strings.Contains(textLower, labelLower)
}

// nearestValueCell finds the next <td>, <span>, or <div> containing a value.
// Rationale: after finding a label anchor, the value is often in a neighboring cell.
func (d *DOM) nearestValueCell(n *html.Node) string {
	// Try to find the next element sibling containing text
	for next := n.NextSibling; next != nil; next = next.NextSibling {
		if next.Type == html.ElementNode {
			switch next.Data {
			case "td", "span", "div", "p":
				if text := d.extractTextContent(next); text != "" {
					return text
				}
			}
		}
	}

	// Try the parent's next sibling
	if n.Parent != nil {
		for next := n.Parent.NextSibling; next != nil; next = next.NextSibling {
			if next.Type == html.ElementNode {
				if text := d.extractTextContent(next); text != "" {
					return text
				}
			}
		}
	}

	return ""
}

// extractTextContent recursively extracts all text from an element.
func (d *DOM) extractTextContent(n *html.Node) string {
	var buf strings.Builder

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node == nil {
			return
		}

		if node.Type == html.TextNode {
			text := strings.TrimSpace(node.Data)
			if text != "" {
				buf.WriteString(text)
				buf.WriteString(" ")
			}
		}

		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(n)
	return strings.TrimSpace(buf.String())
}
