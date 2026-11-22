package downloader

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RedditMediaItem struct {
	URL  string
	Type string
}

type redditPostResponse struct {
	Data struct {
		Children []struct {
			Data struct {
				IsVideo     bool `json:"is_video"`
				SecureMedia *struct {
					RedditVideo *struct {
						FallbackURL string `json:"fallback_url"`
					} `json:"reddit_video"`
				} `json:"secure_media"`
				GalleryData *struct {
					Items []struct {
						MediaID string `json:"media_id"`
					} `json:"items"`
				} `json:"gallery_data"`
				MediaMetadata map[string]struct {
					Status string `json:"status"`
					S      struct {
						URL string `json:"u"`
					} `json:"s"`
				} `json:"media_metadata"`
				Preview *struct {
					Images []struct {
						Source struct {
							URL string `json:"url"`
						} `json:"source"`
					} `json:"images"`
				} `json:"preview"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

func validateRedditURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse Reddit URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("unsupported Reddit URL scheme")
	}
	if parsed.User != nil || parsed.Port() != "" {
		return fmt.Errorf("invalid Reddit URL authority")
	}

	switch strings.ToLower(parsed.Hostname()) {
	case "reddit.com", "www.reddit.com", "old.reddit.com", "new.reddit.com":
		path := parsed.EscapedPath()
		if !strings.Contains(path, "/comments/") && !strings.Contains(path, "/gallery/") && !strings.Contains(path, "/s/") {
			return fmt.Errorf("Reddit post or gallery URL path not found")
		}
	case "redd.it":
		if strings.Trim(parsed.Path, "/") == "" {
			return fmt.Errorf("Reddit short-link code not found")
		}
	default:
		return fmt.Errorf("unsupported Reddit URL host")
	}
	return nil
}

func RedditMedia(rawURL string) ([]RedditMediaItem, error) {
	if err := validateRedditURL(rawURL); err != nil {
		return nil, err
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse Reddit URL: %w", err)
	}
	if strings.EqualFold(parsed.Hostname(), "redd.it") {
		return []RedditMediaItem{{URL: rawURL, Type: "video"}}, nil
	}

	jsonURL := strings.TrimSuffix(rawURL, "/") + ".json"
	request, err := http.NewRequest(http.MethodGet, jsonURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create Reddit request: %w", err)
	}
	request.Header.Set("User-Agent", "lilfella/1.0")

	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Reddit post: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Reddit returned %s", response.Status)
	}

	var payload []redditPostResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Reddit post: %w", err)
	}

	items := parseRedditMedia(payload)
	if len(items) == 0 {
		return nil, fmt.Errorf("Reddit media URL not found")
	}
	return items, nil
}

func parseRedditMedia(payload []redditPostResponse) []RedditMediaItem {
	if len(payload) == 0 {
		return nil
	}

	post := payload[0].Data.Children
	if len(post) == 0 {
		return nil
	}
	data := post[0].Data

	if data.GalleryData != nil {
		items := make([]RedditMediaItem, 0, len(data.GalleryData.Items))
		for _, galleryItem := range data.GalleryData.Items {
			media, ok := data.MediaMetadata[galleryItem.MediaID]
			if !ok || media.Status != "valid" {
				continue
			}
			mediaURL := strings.TrimSpace(html.UnescapeString(media.S.URL))
			if mediaURL != "" {
				items = append(items, RedditMediaItem{URL: mediaURL, Type: "image"})
			}
		}
		if len(items) > 0 {
			return items
		}
	}

	if data.IsVideo {
		return []RedditMediaItem{{Type: "video"}}
	}
	if data.Preview != nil && len(data.Preview.Images) > 0 {
		mediaURL := strings.TrimSpace(html.UnescapeString(data.Preview.Images[0].Source.URL))
		if mediaURL != "" {
			return []RedditMediaItem{{URL: mediaURL, Type: "image"}}
		}
	}
	return nil
}

func RedditVideo(rawURL string) (string, func(), error) {
	if err := validateRedditURL(rawURL); err != nil {
		return "", func() {}, err
	}

	cookiesArgs, err := ytDlpCookiesArgs()
	if err != nil {
		return "", func() {}, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}
	tempDir, err := os.MkdirTemp("", "spotideezer-reddit-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create Reddit temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(id)s.%(ext)s")

	args := append([]string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best", "-o", outputPattern}, ffmpegLocationArgs()...)
	args = append(args, "--merge-output-format", "mp4")
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	command := exec.Command("yt-dlp", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("run yt-dlp for Reddit video: %w: %s", err, output)
	}

	videoPath, err := findFileWithExtension(tempDir, ".mp4")
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	videoPath, compressionCleanup, err := fitVideoToTelegramLimit(videoPath)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	return videoPath, func() {
		compressionCleanup()
		cleanup()
	}, nil
}
