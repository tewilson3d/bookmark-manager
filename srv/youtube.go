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

type YouTubeVideo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"`
	URL         string `json:"url"`
}

func (s *Server) HandleYouTubeImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlaylistURL string `json:"playlist_url"`
		APIKey      string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid JSON", 400)
		return
	}

	playlistID := extractPlaylistID(req.PlaylistURL)
	if playlistID == "" {
		writeError(w, "invalid playlist URL", 400)
		return
	}

	var videos []YouTubeVideo
	var err error

	if req.APIKey != "" {
		// Use official API if key provided
		videos, err = fetchPlaylistWithAPI(playlistID, req.APIKey)
	} else {
		// Scrape without API key
		videos, err = scrapePlaylist(playlistID)
	}

	if err != nil {
		writeError(w, err.Error(), 500)
		return
	}

	// Save videos as bookmarks
	q := dbgen.New(s.DB)
	saved := 0
	for _, v := range videos {
		_, err := q.GetBookmarkByURL(r.Context(), v.URL)
		if err == nil {
			continue // Already exists
		}

		_, err = q.CreateBookmark(r.Context(), dbgen.CreateBookmarkParams{
			Url:         v.URL,
			Title:       v.Title,
			Description: strPtr(v.Description),
			SourceType:  "youtube",
			ImageUrl:    strPtr(v.Thumbnail),
		})
		if err == nil {
			saved++
		}
	}

	writeJSON(w, map[string]any{
		"found":   len(videos),
		"saved":   saved,
		"skipped": len(videos) - saved,
		"videos":  videos,
	})
}

func extractPlaylistID(rawURL string) string {
	// Handle various YouTube playlist URL formats
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// Check for list parameter
	if list := u.Query().Get("list"); list != "" {
		return list
	}

	// Check for /playlist?list= format
	if strings.Contains(u.Path, "playlist") {
		return u.Query().Get("list")
	}

	return ""
}

func fetchPlaylistWithAPI(playlistID, apiKey string) ([]YouTubeVideo, error) {
	var videos []YouTubeVideo
	nextPageToken := ""
	client := &http.Client{Timeout: 30 * time.Second}

	for {
		apiURL := fmt.Sprintf(
			"https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&maxResults=50&playlistId=%s&key=%s",
			playlistID, apiKey,
		)
		if nextPageToken != "" {
			apiURL += "&pageToken=" + nextPageToken
		}

		resp, err := client.Get(apiURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("YouTube API error: %d", resp.StatusCode)
		}

		var result struct {
			Items []struct {
				Snippet struct {
					Title       string `json:"title"`
					Description string `json:"description"`
					ResourceID  struct {
						VideoID string `json:"videoId"`
					} `json:"resourceId"`
					Thumbnails struct {
						Medium struct {
							URL string `json:"url"`
						} `json:"medium"`
					} `json:"thumbnails"`
				} `json:"snippet"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			videoID := item.Snippet.ResourceID.VideoID
			if videoID == "" {
				continue
			}
			videos = append(videos, YouTubeVideo{
				ID:          videoID,
				Title:       item.Snippet.Title,
				Description: truncate(item.Snippet.Description, 200),
				Thumbnail:   item.Snippet.Thumbnails.Medium.URL,
				URL:         "https://www.youtube.com/watch?v=" + videoID,
			})
		}

		nextPageToken = result.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	return videos, nil
}

func scrapePlaylist(playlistID string) ([]YouTubeVideo, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	playlistURL := "https://www.youtube.com/playlist?list=" + playlistID

	req, _ := http.NewRequest("GET", playlistURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Extract video IDs and titles from the page
	var videos []YouTubeVideo

	// Look for ytInitialData JSON
	re := regexp.MustCompile(`var ytInitialData = (\{.*?\});`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		// Try alternative pattern
		re = regexp.MustCompile(`ytInitialData"\s*:\s*(\{.*?\})\s*;`)
		matches = re.FindStringSubmatch(html)
	}

	if len(matches) >= 2 {
		var data map[string]any
		if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
			videos = extractVideosFromInitialData(data)
		}
	}

	// Fallback: regex extraction
	if len(videos) == 0 {
		videoRe := regexp.MustCompile(`"videoId":"([a-zA-Z0-9_-]{11})","thumbnail".*?"title":\{"runs":\[\{"text":"([^"]+)"\}\]`)
		videoMatches := videoRe.FindAllStringSubmatch(html, -1)
		seen := make(map[string]bool)
		for _, m := range videoMatches {
			if len(m) >= 3 && !seen[m[1]] {
				seen[m[1]] = true
				videos = append(videos, YouTubeVideo{
					ID:        m[1],
					Title:     m[2],
					Thumbnail: "https://i.ytimg.com/vi/" + m[1] + "/mqdefault.jpg",
					URL:       "https://www.youtube.com/watch?v=" + m[1],
				})
			}
		}
	}

	if len(videos) == 0 {
		return nil, fmt.Errorf("could not extract videos - playlist may be private or empty. Try using a YouTube API key for better results")
	}

	return videos, nil
}

func extractVideosFromInitialData(data map[string]any) []YouTubeVideo {
	var videos []YouTubeVideo
	seen := make(map[string]bool)

	// Recursively search for video renderers
	var extract func(any)
	extract = func(v any) {
		switch val := v.(type) {
		case map[string]any:
			if renderer, ok := val["playlistVideoRenderer"].(map[string]any); ok {
				videoID, _ := renderer["videoId"].(string)
				if videoID != "" && !seen[videoID] {
					seen[videoID] = true
					title := ""
					if titleObj, ok := renderer["title"].(map[string]any); ok {
						if runs, ok := titleObj["runs"].([]any); ok && len(runs) > 0 {
							if run, ok := runs[0].(map[string]any); ok {
								title, _ = run["text"].(string)
							}
						}
					}
					videos = append(videos, YouTubeVideo{
						ID:        videoID,
						Title:     title,
						Thumbnail: "https://i.ytimg.com/vi/" + videoID + "/mqdefault.jpg",
						URL:       "https://www.youtube.com/watch?v=" + videoID,
					})
				}
			}
			for _, v := range val {
				extract(v)
			}
		case []any:
			for _, v := range val {
				extract(v)
			}
		}
	}
	extract(data)
	return videos
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
