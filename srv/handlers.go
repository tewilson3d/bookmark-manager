package srv

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"srv.exe.dev/db/dbgen"
)

func (s *Server) HandleListBookmarks(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	source := r.URL.Query().Get("source")

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var bookmarks []dbgen.Bookmark
	var err error
	if source != "" {
		bookmarks, err = q.ListBookmarksBySource(r.Context(), dbgen.ListBookmarksBySourceParams{
			SourceType: source, Limit: limit, Offset: offset,
		})
	} else {
		bookmarks, err = q.ListBookmarks(r.Context(), dbgen.ListBookmarksParams{
			Limit: limit, Offset: offset,
		})
	}
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, bookmarks)
}

func (s *Server) HandleCreateBookmark(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL         string   `json:"url"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Summary     string   `json:"summary"`
		SourceType  string   `json:"source_type"`
		FaviconURL  string   `json:"favicon_url"`
		ImageURL    string   `json:"image_url"`
		Tags        []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	if req.URL == "" {
		writeError(w, "url is required", 400)
		return
	}
	if req.SourceType == "" {
		req.SourceType = detectSourceType(req.URL)
	}
	
	// Auto-fetch preview image if not provided
	if req.ImageURL == "" {
		req.ImageURL = getPreviewImage(req.URL)
	}

	q := dbgen.New(s.DB)
	bookmark, err := q.CreateBookmark(r.Context(), dbgen.CreateBookmarkParams{
		Url:         req.URL,
		Title:       req.Title,
		Description: strPtr(req.Description),
		Summary:     strPtr(req.Summary),
		SourceType:  req.SourceType,
		FaviconUrl:  strPtr(req.FaviconURL),
		ImageUrl:    strPtr(req.ImageURL),
	})
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}

	// Add tags
	for _, tagName := range req.Tags {
		tag, err := q.CreateTag(r.Context(), dbgen.CreateTagParams{
			Name: strings.TrimSpace(tagName), Color: strPtr("#6366f1"),
		})
		if err == nil {
			q.AddTagToBookmark(r.Context(), dbgen.AddTagToBookmarkParams{
				BookmarkID: bookmark.ID, TagID: tag.ID,
			})
		}
	}

	w.WriteHeader(201)
	writeJSON(w, bookmark)
}

func (s *Server) HandleGetBookmark(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	q := dbgen.New(s.DB)
	bookmark, err := q.GetBookmark(r.Context(), id)
	if err != nil {
		writeError(w, "not found", 404)
		return
	}
	tags, _ := q.GetBookmarkTags(r.Context(), id)
	writeJSON(w, map[string]any{"bookmark": bookmark, "tags": tags})
}

func (s *Server) HandleUpdateBookmark(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Summary     string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON", 400)
		return
	}
	q := dbgen.New(s.DB)
	bookmark, err := q.UpdateBookmark(r.Context(), dbgen.UpdateBookmarkParams{
		ID: id, Title: req.Title,
		Description: strPtr(req.Description),
		Summary:     strPtr(req.Summary),
	})
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, bookmark)
}

func (s *Server) HandleDeleteBookmark(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	q := dbgen.New(s.DB)
	if err := q.DeleteBookmark(r.Context(), id); err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) HandleListTags(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	tags, err := q.ListTags(r.Context())
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, tags)
}

func (s *Server) HandleCreateTag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	q := dbgen.New(s.DB)
	tag, err := q.CreateTag(r.Context(), dbgen.CreateTagParams{Name: req.Name, Color: &req.Color})
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	w.WriteHeader(201)
	writeJSON(w, tag)
}

func (s *Server) HandleListCollections(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	collections, err := q.ListCollections(r.Context())
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, collections)
}

func (s *Server) HandleCreateCollection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Icon == "" {
		req.Icon = "📁"
	}
	q := dbgen.New(s.DB)
	col, err := q.CreateCollection(r.Context(), dbgen.CreateCollectionParams{
		Name: req.Name, Description: strPtr(req.Description), Icon: strPtr(req.Icon),
	})
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	w.WriteHeader(201)
	writeJSON(w, col)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Server) HandleAnalyzeBookmark(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	q := dbgen.New(s.DB)
	
	bookmark, err := q.GetBookmark(r.Context(), id)
	if err != nil {
		writeError(w, "bookmark not found", 404)
		return
	}
	
	analysis, err := analyzeURL(bookmark.Url)
	if err != nil {
		writeError(w, "failed to analyze: "+err.Error(), 500)
		return
	}
	
	// Update bookmark with analysis
	keywordsJSON, _ := json.Marshal(analysis.Keywords)
	updated, err := q.UpdateBookmarkAnalysis(r.Context(), dbgen.UpdateBookmarkAnalysisParams{
		ID:       id,
		Summary:  &analysis.Summary,
		Keywords: strPtr(string(keywordsJSON)),
	})
	if err != nil {
		writeError(w, "failed to save: "+err.Error(), 500)
		return
	}
	
	writeJSON(w, map[string]any{
		"bookmark": updated,
		"keywords": analysis.Keywords,
	})
}

func detectSourceType(url string) string {
	if strings.Contains(url, "instagram.com") {
		return "instagram"
	}
	if strings.Contains(url, "linkedin.com") {
		return "linkedin"
	}
	if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
		return "youtube"
	}
	return "web"
}

// getPreviewImage fetches og:image or other preview image for a URL
func getPreviewImage(pageURL string) string {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return getScreenshotService(pageURL)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	
	resp, err := client.Do(req)
	if err != nil {
		return getScreenshotService(pageURL)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 100000)) // 100KB should be enough for meta tags
	html := string(body)
	
	// Try og:image first (most reliable for preview)
	ogImage := extractMeta(html, "og:image")
	if ogImage != "" {
		return makeAbsoluteURL(ogImage, pageURL)
	}
	
	// Try twitter:image
	twitterImage := extractMeta(html, "twitter:image")
	if twitterImage != "" {
		return makeAbsoluteURL(twitterImage, pageURL)
	}
	
	// Try twitter:image:src
	twitterImageSrc := extractMeta(html, "twitter:image:src")
	if twitterImageSrc != "" {
		return makeAbsoluteURL(twitterImageSrc, pageURL)
	}
	
	// Fallback to screenshot service
	return getScreenshotService(pageURL)
}

// extractMeta extracts content from meta tags
func extractMeta(html, property string) string {
	// Try property attribute
	patterns := []string{
		`(?i)<meta[^>]+property=["']` + property + `["'][^>]+content=["']([^"']+)["']`,
		`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+property=["']` + property + `["']`,
		`(?i)<meta[^>]+name=["']` + property + `["'][^>]+content=["']([^"']+)["']`,
		`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+name=["']` + property + `["']`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if m := re.FindStringSubmatch(html); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// makeAbsoluteURL converts relative URLs to absolute
func makeAbsoluteURL(imgURL, pageURL string) string {
	if strings.HasPrefix(imgURL, "http://") || strings.HasPrefix(imgURL, "https://") {
		return imgURL
	}
	
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return imgURL
	}
	
	if strings.HasPrefix(imgURL, "//") {
		return parsed.Scheme + ":" + imgURL
	}
	
	if strings.HasPrefix(imgURL, "/") {
		return parsed.Scheme + "://" + parsed.Host + imgURL
	}
	
	return parsed.Scheme + "://" + parsed.Host + "/" + imgURL
}

// getScreenshotService returns a URL for a screenshot/thumbnail service
func getScreenshotService(pageURL string) string {
	// Use thumbnail.ws or similar service for page screenshots
	// This provides a visual preview of the page
	return "https://image.thum.io/get/width/600/crop/400/" + pageURL
}
