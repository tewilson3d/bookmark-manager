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
	tag := r.URL.Query().Get("tag")

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var bookmarks []dbgen.Bookmark
	var err error
	if tag != "" {
		// Query bookmarks by tag name
		rows, qerr := s.DB.QueryContext(r.Context(), `
			SELECT DISTINCT b.id, b.url, b.title, b.description, b.summary, b.source_type, 
			       b.favicon_url, b.image_url, b.created_at, b.updated_at
			FROM bookmarks b
			JOIN bookmark_tags bt ON b.id = bt.bookmark_id
			JOIN tags t ON bt.tag_id = t.id
			WHERE LOWER(t.name) = LOWER(?)
			ORDER BY b.created_at DESC
			LIMIT ? OFFSET ?`, tag, limit, offset)
		if qerr != nil {
			writeError(w, qerr.Error(), 500)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var b dbgen.Bookmark
			if err := rows.Scan(&b.ID, &b.Url, &b.Title, &b.Description, &b.Summary, 
				&b.SourceType, &b.FaviconUrl, &b.ImageUrl, &b.CreatedAt, &b.UpdatedAt); err != nil {
				writeError(w, err.Error(), 500)
				return
			}
			bookmarks = append(bookmarks, b)
		}
	} else if source != "" {
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
	// Default title to URL hostname if not provided
	if req.Title == "" {
		if u, err := url.Parse(req.URL); err == nil {
			req.Title = u.Host
		} else {
			req.Title = req.URL
		}
	}
	if req.SourceType == "" {
		// First check if tags suggest a source type
		if tagSource := detectSourceTypeFromTags(req.Tags); tagSource != "" {
			req.SourceType = tagSource
		} else {
			req.SourceType = detectSourceType(req.URL)
		}
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

func (s *Server) HandleGetCollectionBookmarks(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	q := dbgen.New(s.DB)
	bookmarks, err := q.GetBookmarksInCollection(r.Context(), dbgen.GetBookmarksInCollectionParams{
		CollectionID: id,
		Limit:        1000,
		Offset:       0,
	})
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	writeJSON(w, bookmarks)
}

func (s *Server) HandleAddBookmarksToCollection(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req struct {
		BookmarkIDs []int64 `json:"bookmark_ids"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	q := dbgen.New(s.DB)
	for _, bid := range req.BookmarkIDs {
		q.AddBookmarkToCollection(r.Context(), dbgen.AddBookmarkToCollectionParams{
			BookmarkID:   bid,
			CollectionID: id,
		})
	}
	w.WriteHeader(200)
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) HandleRemoveBookmarksFromCollection(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req struct {
		BookmarkIDs []int64 `json:"bookmark_ids"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	q := dbgen.New(s.DB)
	for _, bid := range req.BookmarkIDs {
		q.RemoveBookmarkFromCollection(r.Context(), dbgen.RemoveBookmarkFromCollectionParams{
			BookmarkID:   bid,
			CollectionID: id,
		})
	}
	w.WriteHeader(200)
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) HandleBulkUpdateBookmarks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BookmarkIDs []int64 `json:"bookmark_ids"`
		SourceType  string  `json:"source_type"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	
	for _, bid := range req.BookmarkIDs {
		s.DB.ExecContext(r.Context(), "UPDATE bookmarks SET source_type = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", req.SourceType, bid)
	}
	w.WriteHeader(200)
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) HandleGenerateAllMetadata(w http.ResponseWriter, r *http.Request) {
	q := dbgen.New(s.DB)
	
	// Get all bookmarks
	bookmarks, err := q.ListBookmarks(r.Context(), dbgen.ListBookmarksParams{
		Limit: 1000, Offset: 0,
	})
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	
	updated := 0
	for _, b := range bookmarks {
		needsUpdate := false
		newImageURL := b.ImageUrl
		newSummary := b.Summary
		
		// Check if preview image is missing
		if b.ImageUrl == nil || *b.ImageUrl == "" {
			img := getPreviewImage(b.Url)
			if img != "" {
				newImageURL = &img
				needsUpdate = true
			}
		}
		
		// Check if summary is missing
		if b.Summary == nil || *b.Summary == "" {
			analysis, err := analyzeURL(b.Url)
			if err == nil && analysis.Summary != "" {
				newSummary = &analysis.Summary
				needsUpdate = true
			}
		}
		
		if needsUpdate {
			// Update the bookmark
			_, err := s.DB.ExecContext(r.Context(), `
				UPDATE bookmarks SET 
					image_url = COALESCE(?, image_url),
					summary = COALESCE(?, summary),
					updated_at = CURRENT_TIMESTAMP
				WHERE id = ?`,
				newImageURL, newSummary, b.ID)
			if err == nil {
				updated++
			}
		}
	}
	
	writeJSON(w, map[string]any{
		"total":   len(bookmarks),
		"updated": updated,
	})
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
	
	// If title is empty or just hostname, generate from summary
	needsTitle := bookmark.Title == ""
	if !needsTitle {
		if u, err := url.Parse(bookmark.Url); err == nil {
			needsTitle = bookmark.Title == u.Host
		}
	}
	if needsTitle && analysis.Summary != "" {
		// Generate a short title from summary (first sentence, max 60 chars)
		title := analysis.Summary
		if idx := strings.Index(title, "."); idx > 0 && idx < 80 {
			title = title[:idx]
		} else if len(title) > 60 {
			title = title[:57] + "..."
		}
		s.DB.ExecContext(r.Context(), "UPDATE bookmarks SET title = ? WHERE id = ?", title, id)
		updated.Title = title
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

// detectSourceTypeFromTags checks tags and returns an appropriate source type
// Returns empty string if no matching tags found
func detectSourceTypeFromTags(tags []string) string {
	for _, tag := range tags {
		lowerTag := strings.ToLower(strings.TrimSpace(tag))
		switch lowerTag {
		case "blender":
			return "blender"
		case "maya":
			return "maya"
		case "unreal":
			return "unreal"
		case "rig", "rigging", "model", "models":
			return "models"
		case "material", "materials", "texture", "textures", "shader", "shaders":
			return "materials"
		case "job", "jobs":
			return "jobs"
		}
	}
	return ""
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

func (s *Server) HandleAddTagToBookmark(w http.ResponseWriter, r *http.Request) {
	bidStr := r.PathValue("id")
	tagName := r.PathValue("tag")
	
	bid, err := strconv.ParseInt(bidStr, 10, 64)
	if err != nil {
		writeError(w, "invalid bookmark id", 400)
		return
	}
	
	// Create or get the tag
	q := dbgen.New(s.DB)
	tag, err := q.CreateTag(r.Context(), dbgen.CreateTagParams{
		Name:  strings.TrimSpace(tagName),
		Color: strPtr("#8b5cf6"),
	})
	if err != nil {
		// Tag might already exist, try to get it
		tags, _ := q.ListTags(r.Context())
		for _, t := range tags {
			if strings.EqualFold(t.Name, tagName) {
				tag = t
				break
			}
		}
	}
	
	if tag.ID == 0 {
		writeError(w, "could not create or find tag", 500)
		return
	}
	
	// Add the tag to the bookmark (ignore if already exists)
	_, _ = s.DB.ExecContext(r.Context(), 
		"INSERT OR IGNORE INTO bookmark_tags (bookmark_id, tag_id) VALUES (?, ?)",
		bid, tag.ID)
	
	writeJSON(w, map[string]any{"success": true, "tag": tag.Name})
}

func (s *Server) HandleRemoveTagFromBookmark(w http.ResponseWriter, r *http.Request) {
	bidStr := r.PathValue("id")
	tagName := r.PathValue("tag")
	
	bid, err := strconv.ParseInt(bidStr, 10, 64)
	if err != nil {
		writeError(w, "invalid bookmark id", 400)
		return
	}
	
	// Find the tag
	q := dbgen.New(s.DB)
	tags, _ := q.ListTags(r.Context())
	var tagID int64
	for _, t := range tags {
		if strings.EqualFold(t.Name, tagName) {
			tagID = t.ID
			break
		}
	}
	
	if tagID == 0 {
		writeError(w, "tag not found", 404)
		return
	}
	
	// Remove the tag from the bookmark
	_, err = s.DB.ExecContext(r.Context(), 
		"DELETE FROM bookmark_tags WHERE bookmark_id = ? AND tag_id = ?",
		bid, tagID)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	
	writeJSON(w, map[string]any{"success": true, "removed": tagName})
}

func (s *Server) HandleRemoveDuplicates(w http.ResponseWriter, r *http.Request) {
	// Find and remove duplicate bookmarks (same URL), keeping the oldest one
	result, err := s.DB.ExecContext(r.Context(), `
		DELETE FROM bookmarks 
		WHERE id NOT IN (
			SELECT MIN(id) FROM bookmarks GROUP BY url
		)
	`)
	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}
	
	deleted, _ := result.RowsAffected()
	writeJSON(w, map[string]any{"success": true, "deleted": deleted})
}

func (s *Server) HandleAutoCategorizeTags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	updated := make(map[string]int64)
	
	// Tag patterns for each source type
	categories := []struct {
		SourceType string
		Patterns   []string
		Exact      []string
	}{
		{"blender", []string{"%blender%"}, nil},
		{"maya", []string{"%maya%"}, nil},
		{"unreal", []string{"%unreal%", "%metahuman%", "%merahuman%"}, nil},
		{"models", nil, []string{"rig", "rigging", "model", "models", "modeling", "mocap", "movap", "animation", "hair", "hair groom", "clothing", "hamds"}},
		{"materials", []string{"%texture%", "%shader%"}, []string{"material", "materials"}},
		{"ai", []string{"%ai%"}, []string{"comfy", "comfy rig", "prompt", "model gen", "seedance"}},
		{"3d", []string{"%3d%"}, []string{"houdini", "unity"}},
		{"jobs", nil, []string{"job", "jobs"}},
	}
	
	for _, cat := range categories {
		var conditions []string
		var args []interface{}
		
		for _, pattern := range cat.Patterns {
			conditions = append(conditions, "LOWER(t.name) LIKE ?")
			args = append(args, pattern)
		}
		for _, exact := range cat.Exact {
			conditions = append(conditions, "LOWER(t.name) = ?")
			args = append(args, exact)
		}
		
		if len(conditions) == 0 {
			continue
		}
		
		query := `
			UPDATE bookmarks SET source_type = ? 
			WHERE id IN (
				SELECT DISTINCT bt.bookmark_id FROM bookmark_tags bt
				JOIN tags t ON bt.tag_id = t.id
				WHERE ` + strings.Join(conditions, " OR ") + `
			)`
		
		allArgs := append([]interface{}{cat.SourceType}, args...)
		result, err := s.DB.ExecContext(ctx, query, allArgs...)
		if err == nil {
			count, _ := result.RowsAffected()
			updated[cat.SourceType] = count
		}
	}
	
	writeJSON(w, map[string]any{"success": true, "updated": updated})
}
