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

	// Extract text content
	text := extractText(html)

	// Generate summary
	summary := generateSummary(html, text)

	// Extract keywords
	keywords := extractKeywords(text)

	return &ContentAnalysis{
		Summary:  summary,
		Keywords: keywords,
	}, nil
}

func extractText(html string) string {
	// Remove script and style tags
	re := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = re.ReplaceAllString(html, " ")
	re = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = re.ReplaceAllString(html, " ")
	re = regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
	html = re.ReplaceAllString(html, " ")
	re = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	html = re.ReplaceAllString(html, " ")
	re = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
	html = re.ReplaceAllString(html, " ")

	// Remove all HTML tags
	re = regexp.MustCompile(`<[^>]+>`)
	text := re.ReplaceAllString(html, " ")

	// Decode HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Normalize whitespace
	re = regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

func generateSummary(html, text string) string {
	var summary string

	// Try to get meta description first
	re := regexp.MustCompile(`(?i)<meta[^>]+name=["']description["'][^>]+content=["']([^"']+)["']`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		summary = strings.TrimSpace(m[1])
	}

	// Try og:description
	if summary == "" {
		re = regexp.MustCompile(`(?i)<meta[^>]+property=["']og:description["'][^>]+content=["']([^"']+)["']`)
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			summary = strings.TrimSpace(m[1])
		}
	}

	// Try to find first paragraph
	if summary == "" {
		re = regexp.MustCompile(`(?is)<p[^>]*>([^<]{50,500})</p>`)
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			// Clean the paragraph
			p := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(m[1], "")
			p = strings.TrimSpace(p)
			if len(p) > 50 {
				summary = p
			}
		}
	}

	// Fall back to first part of text
	if summary == "" && len(text) > 100 {
		// Find first sentence or chunk
		end := 300
		if len(text) < end {
			end = len(text)
		}
		summary = text[:end]
		// Try to end at a sentence
		if idx := strings.LastIndexAny(summary, ".!?"); idx > 100 {
			summary = summary[:idx+1]
		}
	}

	// Truncate if too long
	if len(summary) > 500 {
		summary = summary[:500] + "..."
	}

	return summary
}

func extractKeywords(text string) []string {
	// Tokenize and count words
	wordCounts := make(map[string]int)
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) < 3 || len(word) > 30 {
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
