package downloader

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type RedditMediaItem struct {
	URL  string
	Type string
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
	cookiesArgs, err := ytDlpCookiesArgs()
	if err != nil {
		return nil, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}

	args := append([]string{"--dump-single-json", "--no-warnings", "--skip-download"}, cookiesArgs...)
	args = append(args, rawURL)
	output, err := exec.Command("yt-dlp", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect Reddit media with yt-dlp: %w: %s", err, output)
	}

	return parseRedditYTDlpMedia(output)
}

type redditYTDlpEntry struct {
	URL       string             `json:"url"`
	Ext       string             `json:"ext"`
	Vcodec    string             `json:"vcodec"`
	Entries   []redditYTDlpEntry `json:"entries"`
	Thumbnail string             `json:"thumbnail"`
}

func parseRedditYTDlpMedia(output []byte) ([]RedditMediaItem, error) {
	var data redditYTDlpEntry
	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("decode yt-dlp Reddit metadata: %w", err)
	}

	sourceEntries := data.Entries
	if len(sourceEntries) == 0 {
		sourceEntries = []redditYTDlpEntry{data}
	}

	items := make([]RedditMediaItem, 0, len(sourceEntries))
	seen := make(map[string]bool)
	for _, entry := range sourceEntries {
		rawMediaURL := strings.TrimSpace(entry.URL)
		if rawMediaURL == "" {
			rawMediaURL = strings.TrimSpace(entry.Thumbnail)
		}
		if rawMediaURL == "" || seen[rawMediaURL] {
			continue
		}
		parsed, err := url.Parse(rawMediaURL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			continue
		}

		mediaType := inferRedditMediaType(entry)
		items = append(items, RedditMediaItem{URL: rawMediaURL, Type: mediaType})
		seen[rawMediaURL] = true
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("Reddit media URL not found")
	}
	return items, nil
}

func inferRedditMediaType(entry redditYTDlpEntry) string {
	ext := strings.ToLower(strings.TrimPrefix(entry.Ext, "."))
	if ext == "jpg" || ext == "jpeg" || ext == "png" || ext == "gif" || ext == "webp" {
		return "image"
	}
	if entry.Vcodec != "" && !strings.EqualFold(entry.Vcodec, "none") {
		return "video"
	}
	if strings.HasPrefix(strings.ToLower(entry.URL), "http") {
		parsed, err := url.Parse(entry.URL)
		if err == nil {
			host := strings.ToLower(parsed.Hostname())
			pathExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(parsed.Path), "."))
			if host == "i.redd.it" || pathExt == "jpg" || pathExt == "jpeg" || pathExt == "png" || pathExt == "gif" || pathExt == "webp" {
				return "image"
			}
			if pathExt == "mp4" || pathExt == "webm" || pathExt == "m3u8" {
				return "video"
			}
		}
	}
	return "video"
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
