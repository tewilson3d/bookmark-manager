package srv

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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
