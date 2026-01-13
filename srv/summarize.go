package srv

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Common stop words to filter out
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "as": true, "is": true, "was": true,
	"are": true, "were": true, "been": true, "be": true, "have": true, "has": true,
	"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
	"could": true, "should": true, "may": true, "might": true, "must": true,
	"shall": true, "can": true, "need": true, "dare": true, "ought": true,
	"used": true, "this": true, "that": true, "these": true, "those": true,
	"i": true, "you": true, "he": true, "she": true, "it": true, "we": true, "they": true,
	"what": true, "which": true, "who": true, "whom": true, "whose": true,
	"where": true, "when": true, "why": true, "how": true, "all": true, "each": true,
	"every": true, "both": true, "few": true, "more": true, "most": true, "other": true,
	"some": true, "such": true, "no": true, "nor": true, "not": true, "only": true,
	"own": true, "same": true, "so": true, "than": true, "too": true, "very": true,
	"just": true, "also": true, "now": true, "here": true, "there": true, "then": true,
	"once": true, "about": true, "into": true, "through": true, "during": true,
	"before": true, "after": true, "above": true, "below": true, "between": true,
	"under": true, "again": true, "further": true, "if": true, "because": true,
	"until": true, "while": true, "your": true, "our": true, "their": true, "my": true,
	"its": true, "his": true, "her": true, "up": true, "down": true, "out": true,
	"off": true, "over": true, "any": true, "get": true, "got": true, "like": true,
	"make": true, "made": true, "new": true, "one": true, "two": true, "first": true,
	"even": true, "way": true, "well": true, "back": true, "being": true, "want": true,
	"use": true, "using": true, "click": true, "page": true, "website": true,
	"http": true, "https": true, "www": true, "com": true, "org": true, "net": true,
	"function": true, "window": true, "var": true, "const": true, "let": true,
	"return": true, "true": true, "false": true, "null": true, "undefined": true,
}

type ContentAnalysis struct {
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
}

func (s *Server) HandleAnalyzeURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON", 400)
		return
	}

	analysis, err := analyzeURL(req.URL)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}

	writeJSON(w, analysis)
}

func analyzeURL(url string) (*ContentAnalysis, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 500000)) // 500KB max
	html := string(body)

	// Generate summary from metadata and clean content
	summary := generateSummary(html, url)

	// Extract text for keywords
	text := extractText(html)
	keywords := extractKeywords(text)

	return &ContentAnalysis{
		Summary:  summary,
		Keywords: keywords,
	}, nil
}

func extractText(html string) string {
	// Remove script tags and their content
	re := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = re.ReplaceAllString(html, " ")
	
	// Remove style tags
	re = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = re.ReplaceAllString(html, " ")
	
	// Remove noscript
	re = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	html = re.ReplaceAllString(html, " ")
	
	// Remove JSON-LD
	re = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>.*?</script>`)
	html = re.ReplaceAllString(html, " ")

	// Remove nav, footer, header, aside
	for _, tag := range []string{"nav", "footer", "header", "aside", "menu"} {
		re = regexp.MustCompile(`(?is)<` + tag + `[^>]*>.*?</` + tag + `>`)
		html = re.ReplaceAllString(html, " ")
	}

	// Remove all HTML tags
	re = regexp.MustCompile(`<[^>]+>`)
	text := re.ReplaceAllString(html, " ")

	// Decode HTML entities
	text = decodeHTMLEntities(text)

	// Remove any remaining JavaScript-like content
	re = regexp.MustCompile(`\{[^}]*\}`)
	text = re.ReplaceAllString(text, " ")
	re = regexp.MustCompile(`\[[^\]]*\]`)
	text = re.ReplaceAllString(text, " ")

	// Normalize whitespace
	re = regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func decodeHTMLEntities(text string) string {
	replacements := map[string]string{
		"&nbsp;": " ", "&amp;": "&", "&lt;": "<", "&gt;": ">",
		"&quot;": "\"", "&#39;": "'", "&apos;": "'",
		"&mdash;": "—", "&ndash;": "–", "&hellip;": "...",
		"&copy;": "©", "&reg;": "®", "&trade;": "™",
	}
	for entity, char := range replacements {
		text = strings.ReplaceAll(text, entity, char)
	}
	// Remove numeric entities
	re := regexp.MustCompile(`&#\d+;`)
	text = re.ReplaceAllString(text, " ")
	return text
}

func generateSummary(html, url string) string {
	var parts []string

	// 1. Get the title
	title := extractMetaContent(html, "og:title")
	if title == "" {
		title = extractMetaContent(html, "twitter:title")
	}
	if title == "" {
		re := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			title = strings.TrimSpace(m[1])
		}
	}

	// 2. Get description
	description := extractMetaContent(html, "og:description")
	if description == "" {
		description = extractMetaContent(html, "description")
	}
	if description == "" {
		description = extractMetaContent(html, "twitter:description")
	}

	// 3. Get site name
	siteName := extractMetaContent(html, "og:site_name")
	if siteName == "" {
		siteName = extractMetaContent(html, "application-name")
	}

	// 4. Get type/category
	contentType := extractMetaContent(html, "og:type")

	// 5. Get author
	author := extractMetaContent(html, "author")
	if author == "" {
		author = extractMetaContent(html, "article:author")
	}

	// 6. Get publish date
	publishDate := extractMetaContent(html, "article:published_time")
	if publishDate == "" {
		publishDate = extractMetaContent(html, "datePublished")
	}

	// Build human-readable summary
	if siteName != "" && siteName != title {
		parts = append(parts, "From "+siteName+".")
	}

	if contentType != "" && contentType != "website" {
		parts = append(parts, strings.Title(strings.ReplaceAll(contentType, "_", " "))+".")
	}

	if description != "" {
		// Clean up description
		description = decodeHTMLEntities(description)
		description = strings.TrimSpace(description)
		if len(description) > 400 {
			description = description[:400] + "..."
		}
		parts = append(parts, description)
	}

	if author != "" {
		parts = append(parts, "By "+author+".")
	}

	if publishDate != "" {
		// Try to format date nicely
		if t, err := time.Parse(time.RFC3339, publishDate); err == nil {
			parts = append(parts, "Published "+t.Format("January 2, 2006")+".")
		} else if t, err := time.Parse("2006-01-02", publishDate); err == nil {
			parts = append(parts, "Published "+t.Format("January 2, 2006")+".")
		}
	}

	// If we still don't have a good description, try to get first paragraph
	if description == "" {
		firstPara := extractFirstParagraph(html)
		if firstPara != "" {
			parts = append(parts, firstPara)
		}
	}

	// Detect content type from URL if not specified
	if len(parts) == 0 || (len(parts) == 1 && siteName != "") {
		urlLower := strings.ToLower(url)
		switch {
		case strings.Contains(urlLower, "youtube.com") || strings.Contains(urlLower, "youtu.be"):
			videoTitle := extractMetaContent(html, "og:title")
			if videoTitle != "" {
				parts = append(parts, "YouTube video: "+videoTitle)
			}
		case strings.Contains(urlLower, "instagram.com"):
			parts = append(parts, "Instagram post.")
		case strings.Contains(urlLower, "linkedin.com"):
			parts = append(parts, "LinkedIn content.")
		case strings.Contains(urlLower, "twitter.com") || strings.Contains(urlLower, "x.com"):
			parts = append(parts, "Twitter/X post.")
		case strings.Contains(urlLower, "github.com"):
			parts = append(parts, "GitHub repository or page.")
		}
	}

	summary := strings.Join(parts, " ")
	
	// Final cleanup
	summary = strings.TrimSpace(summary)
	
	// Make sure we don't have JavaScript garbage
	if strings.Contains(summary, "function") || strings.Contains(summary, "window.") || 
	   strings.Contains(summary, "{") || strings.Contains(summary, "var ") ||
	   strings.Contains(summary, "ytcfg") || strings.Contains(summary, "ytplayer") {
		// Fall back to just title + site
		parts = []string{}
		if siteName != "" {
			parts = append(parts, "From "+siteName+".")
		}
		if title != "" {
			parts = append(parts, title)
		}
		summary = strings.Join(parts, " ")
	}

	if summary == "" {
		summary = "No description available for this page."
	}

	return summary
}

func extractMetaContent(html, name string) string {
	// Try property attribute (og:, twitter:)
	patterns := []string{
		`(?i)<meta[^>]+property=["']` + regexp.QuoteMeta(name) + `["'][^>]+content=["']([^"']+)["']`,
		`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']` + regexp.QuoteMeta(name) + `["']`,
		`(?i)<meta[^>]+name=["']` + regexp.QuoteMeta(name) + `["'][^>]+content=["']([^"']+)["']`,
		`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+name=["']` + regexp.QuoteMeta(name) + `["']`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			return strings.TrimSpace(decodeHTMLEntities(m[1]))
		}
	}
	return ""
}

func extractFirstParagraph(html string) string {
	// Look for article content first
	re := regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		html = m[1]
	}

	// Find first meaningful paragraph
	re = regexp.MustCompile(`(?is)<p[^>]*>([^<]{100,})</p>`)
	matches := re.FindAllStringSubmatch(html, 5)
	
	for _, m := range matches {
		if len(m) > 1 {
			text := strings.TrimSpace(m[1])
			text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, "")
			text = decodeHTMLEntities(text)
			text = strings.TrimSpace(text)
			
			// Skip if it looks like code or garbage
			if strings.Contains(text, "{") || strings.Contains(text, "function") ||
			   strings.Contains(text, "var ") || len(text) < 50 {
				continue
			}
			
			if len(text) > 300 {
				text = text[:300] + "..."
			}
			return text
		}
	}
	return ""
}

func extractKeywords(text string) []string {
	// Skip if text looks like code
	if strings.Contains(text, "function") || strings.Contains(text, "window.") {
		return []string{}
	}

	// Tokenize and count words
	wordCounts := make(map[string]int)
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) < 3 || len(word) > 25 {
			continue
		}
		if stopWords[word] {
			continue
		}
		// Skip if mostly numbers
		numCount := 0
		for _, r := range word {
			if unicode.IsNumber(r) {
				numCount++
			}
		}
		if numCount > len(word)/2 {
			continue
		}
		// Skip common code words
		codeWords := []string{"script", "style", "div", "span", "class", "href", "src", "img", "onclick", "onload"}
		isCode := false
		for _, cw := range codeWords {
			if word == cw {
				isCode = true
				break
			}
		}
		if isCode {
			continue
		}
		wordCounts[word]++
	}

	// Sort by frequency
	type wordFreq struct {
		word  string
		count int
	}
	var freqs []wordFreq
	for word, count := range wordCounts {
		if count >= 2 { // Must appear at least twice
			freqs = append(freqs, wordFreq{word, count})
		}
	}
	sort.Slice(freqs, func(i, j int) bool {
		return freqs[i].count > freqs[j].count
	})

	// Take top keywords
	var keywords []string
	for i, wf := range freqs {
		if i >= 15 { // Max 15 keywords
			break
		}
		keywords = append(keywords, wf.word)
	}

	return keywords
}


