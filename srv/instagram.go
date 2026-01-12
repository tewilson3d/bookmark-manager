package srv

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"srv.exe.dev/db/dbgen"
)

// Instagram data export format
type InstagramExport struct {
	SavedSavedMedia []struct {
		Title string `json:"title"`
		StringListData []struct {
			Href      string `json:"href"`
			Value     string `json:"value"`
			Timestamp int64  `json:"timestamp"`
		} `json:"string_list_data"`
	} `json:"saved_saved_media"`
}

// Alternative format (newer exports)
type InstagramExportAlt struct {
	SavedPosts []struct {
		MediaURL  string `json:"media_url"`
		Title     string `json:"title"`
		Timestamp int64  `json:"timestamp"`
	} `json:"saved_posts"`
}

func (s *Server) HandleInstagramImport(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, "File too large or invalid form", 400)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, "No file uploaded", 400)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, "Could not read file", 500)
		return
	}

	// Try to parse as Instagram export JSON
	var urls []string
	var titles []string

	// Try first format
	var export InstagramExport
	if err := json.Unmarshal(data, &export); err == nil && len(export.SavedSavedMedia) > 0 {
		for _, item := range export.SavedSavedMedia {
			for _, link := range item.StringListData {
				if link.Href != "" && strings.Contains(link.Href, "instagram.com") {
					urls = append(urls, link.Href)
					titles = append(titles, item.Title)
				}
			}
		}
	}

	// Try alternative format
	if len(urls) == 0 {
		var exportAlt InstagramExportAlt
		if err := json.Unmarshal(data, &exportAlt); err == nil {
			for _, post := range exportAlt.SavedPosts {
				if post.MediaURL != "" {
					urls = append(urls, post.MediaURL)
					titles = append(titles, post.Title)
				}
			}
		}
	}

	// Try parsing as array of URLs or generic JSON with href fields
	if len(urls) == 0 {
		// Try as simple array of strings
		var simpleUrls []string
		if err := json.Unmarshal(data, &simpleUrls); err == nil {
			for _, u := range simpleUrls {
				if strings.Contains(u, "instagram.com") {
					urls = append(urls, u)
					titles = append(titles, "Instagram Post")
				}
			}
		}
	}

	// Try to find any instagram URLs in the raw JSON
	if len(urls) == 0 {
		// Generic extraction - find all instagram.com URLs in the JSON
		var generic any
		if err := json.Unmarshal(data, &generic); err == nil {
			urls = extractInstagramURLs(generic)
			for range urls {
				titles = append(titles, "Instagram Post")
			}
		}
	}

	if len(urls) == 0 {
		writeError(w, "No Instagram URLs found in file. Make sure you uploaded the correct JSON file from Instagram data export.", 400)
		return
	}

	// Save bookmarks
	q := dbgen.New(s.DB)
	saved := 0
	for i, url := range urls {
		// Check if already exists
		_, err := q.GetBookmarkByURL(r.Context(), url)
		if err == nil {
			continue
		}

		title := "Instagram Post"
		if i < len(titles) && titles[i] != "" {
			title = titles[i]
		}

		_, err = q.CreateBookmark(r.Context(), dbgen.CreateBookmarkParams{
			Url:        url,
			Title:      title,
			SourceType: "instagram",
		})
		if err == nil {
			saved++
		}
	}

	writeJSON(w, map[string]any{
		"found":   len(urls),
		"saved":   saved,
		"skipped": len(urls) - saved,
	})
}

func extractInstagramURLs(v any) []string {
	var urls []string
	seen := make(map[string]bool)

	var extract func(any)
	extract = func(val any) {
		switch v := val.(type) {
		case string:
			if strings.Contains(v, "instagram.com/p/") || strings.Contains(v, "instagram.com/reel/") {
				if !seen[v] {
					seen[v] = true
					urls = append(urls, v)
				}
			}
		case map[string]any:
			for _, val := range v {
				extract(val)
			}
		case []any:
			for _, val := range v {
				extract(val)
			}
		}
	}
	extract(v)
	return urls
}
