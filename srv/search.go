package srv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"srv.exe.dev/db/dbgen"
)

func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, "query required", 400)
		return
	}

	// Search including keywords
	like := "%" + query + "%"
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT * FROM bookmarks 
		WHERE title LIKE ? 
		   OR description LIKE ? 
		   OR summary LIKE ? 
		   OR keywords LIKE ?
		ORDER BY created_at DESC
		LIMIT 50
	`, like, like, like, like)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var bookmarks []dbgen.Bookmark
	for rows.Next() {
		var b dbgen.Bookmark
		var keywords *string
		if err := rows.Scan(&b.ID, &b.Url, &b.Title, &b.Description, &b.Summary,
			&b.SourceType, &b.FaviconUrl, &b.ImageUrl, &b.CreatedAt, &b.UpdatedAt, &keywords); err == nil {
			bookmarks = append(bookmarks, b)
		}
	}
	writeJSON(w, map[string]any{"bookmarks": bookmarks})
}

type Metadata struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Favicon     string `json:"favicon"`
	SourceType  string `json:"source_type"`
}

func (s *Server) HandleFetchMetadata(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON", 400)
		return
	}

	meta, err := fetchMetadata(req.URL)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, meta)
}

func fetchMetadata(rawURL string) (*Metadata, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	html := string(body)

	parsedURL, _ := url.Parse(rawURL)
	meta := &Metadata{
		SourceType: detectSourceType(rawURL),
		Favicon:    fmt.Sprintf("%s://%s/favicon.ico", parsedURL.Scheme, parsedURL.Host),
	}

	// Extract title
	if m := regexp.MustCompile(`<title[^>]*>([^<]+)</title>`).FindStringSubmatch(html); len(m) > 1 {
		meta.Title = strings.TrimSpace(m[1])
	}

	// Extract og:title
	if m := regexp.MustCompile(`<meta[^>]+property=["']og:title["'][^>]+content=["']([^"']+)["']`).FindStringSubmatch(html); len(m) > 1 {
		meta.Title = strings.TrimSpace(m[1])
	}

	// Extract description
	if m := regexp.MustCompile(`<meta[^>]+name=["']description["'][^>]+content=["']([^"']+)["']`).FindStringSubmatch(html); len(m) > 1 {
		meta.Description = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`<meta[^>]+property=["']og:description["'][^>]+content=["']([^"']+)["']`).FindStringSubmatch(html); len(m) > 1 {
		meta.Description = strings.TrimSpace(m[1])
	}

	// Extract og:image
	if m := regexp.MustCompile(`<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`).FindStringSubmatch(html); len(m) > 1 {
		meta.Image = strings.TrimSpace(m[1])
	}

	return meta, nil
}

// HandleWebSearch searches the internet for similar content
func (s *Server) HandleWebSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, "query required", 400)
		return
	}

	// Use DuckDuckGo instant answers API (no API key needed)
	client := &http.Client{Timeout: 10 * time.Second}
	searchURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1", url.QueryEscape(query))
	resp, err := client.Get(searchURL)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	var ddgResp struct {
		Abstract     string `json:"Abstract"`
		AbstractURL  string `json:"AbstractURL"`
		AbstractText string `json:"AbstractText"`
		Heading      string `json:"Heading"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	json.NewDecoder(resp.Body).Decode(&ddgResp)

	results := []map[string]string{}
	
	if ddgResp.Abstract != "" {
		results = append(results, map[string]string{
			"title":       ddgResp.Heading,
			"description": ddgResp.AbstractText,
			"url":         ddgResp.AbstractURL,
		})
	}

	for _, topic := range ddgResp.RelatedTopics {
		if topic.Text != "" && topic.FirstURL != "" {
			results = append(results, map[string]string{
				"title":       topic.Text,
				"description": "",
				"url":         topic.FirstURL,
			})
		}
		if len(results) >= 10 {
			break
		}
	}

	writeJSON(w, map[string]any{"results": results, "search_url": "https://duckduckgo.com/?q=" + url.QueryEscape(query)})
}
