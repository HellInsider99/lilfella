package downloader

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var instagramPostPathPattern = regexp.MustCompile(`^/(?:reels?|p)/([A-Za-z0-9_-]+)/?$`)

type InstagramMediaItem struct {
	Path string
	Type string // "image" or "video"
}

// InstagramPost is the downloaded media of a post in post order, plus the
// post's caption (empty if it has none or it could not be read).
type InstagramPost struct {
	Items       []InstagramMediaItem
	Description string
}

// InstagramMedia downloads every image and video of an Instagram post, reel or
// carousel using gallery-dl, along with the post caption. If gallery-dl fails,
// it falls back to yt-dlp, which can only fetch videos.
func InstagramMedia(rawURL string) (InstagramPost, func(), error) {
	if instagramShortcode(rawURL) == "" {
		return InstagramPost{}, func() {}, fmt.Errorf("Instagram shortcode not found")
	}

	post, cleanup, galleryErr := instagramGalleryDLMedia(rawURL)
	if galleryErr == nil {
		return post, cleanup, nil
	}

	videoPath, description, videoCleanup, ytErr := instagramYtDlpVideo(rawURL)
	if ytErr != nil {
		return InstagramPost{}, func() {}, fmt.Errorf("gallery-dl: %v; yt-dlp fallback: %w", galleryErr, ytErr)
	}
	return InstagramPost{
		Items:       []InstagramMediaItem{{Path: videoPath, Type: "video"}},
		Description: description,
	}, videoCleanup, nil
}

func instagramGalleryDLMedia(rawURL string) (InstagramPost, func(), error) {
	cookiesArgs, cleanupCookies, err := instagramYtDlpCookiesArgs()
	if err != nil {
		return InstagramPost{}, func() {}, fmt.Errorf("configure gallery-dl cookies: %w", err)
	}
	defer cleanupCookies()

	tempDir, err := os.MkdirTemp("", "lilfella-instagram-")
	if err != nil {
		return InstagramPost{}, func() {}, fmt.Errorf("create Instagram temp directory: %w", err)
	}
	cleanups := []func(){func() { _ = os.RemoveAll(tempDir) }}
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	// cookiesArgs is either empty or {"--cookies", path}; gallery-dl uses the same flag.
	// --write-metadata saves a "<file>.json" next to each file; it holds the caption.
	args := []string{"--directory", tempDir, "--filename", "{num:>03}.{extension}", "--write-metadata"}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	command := exec.Command("gallery-dl", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return InstagramPost{}, func() {}, fmt.Errorf("run gallery-dl for Instagram post: %w: %s", err, output)
	}

	items, err := collectInstagramMedia(tempDir)
	if err != nil {
		cleanup()
		return InstagramPost{}, func() {}, err
	}
	description := readInstagramDescription(tempDir)

	for i, item := range items {
		if item.Type != "video" {
			continue
		}
		fittedPath, compressionCleanup, err := fitVideoToTelegramLimit(item.Path)
		if err != nil {
			cleanup()
			return InstagramPost{}, func() {}, err
		}
		cleanups = append(cleanups, compressionCleanup)
		items[i].Path = fittedPath
	}
	return InstagramPost{Items: items, Description: description}, cleanup, nil
}

// readInstagramDescription returns the post caption from the first metadata
// JSON file in dir that has one (gallery-dl's "<file>.json" or yt-dlp's
// ".info.json"; both use the "description" key). It returns "" if none is found.
func readInstagramDescription(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var metadata struct {
			Description string `json:"description"`
		}
		if json.Unmarshal(data, &metadata) != nil {
			continue
		}
		if description := strings.TrimSpace(metadata.Description); description != "" {
			return description
		}
	}
	return ""
}

// collectInstagramMedia returns downloaded media files in dir, sorted by name
// (gallery-dl numbers them in carousel order), classified as image or video.
func collectInstagramMedia(dir string) ([]InstagramMediaItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("scan downloaded media: %w", err)
	}

	var items []InstagramMediaItem
	for _, entry := range entries { // os.ReadDir returns entries sorted by name
		if !entry.Type().IsRegular() {
			continue
		}
		var mediaType string
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".jpg", ".jpeg", ".png", ".webp", ".heic":
			mediaType = "image"
		case ".mp4", ".mov":
			mediaType = "video"
		default:
			continue
		}
		items = append(items, InstagramMediaItem{Path: filepath.Join(dir, entry.Name()), Type: mediaType})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no downloaded Instagram media found")
	}
	return items, nil
}

func InstagramReel(rawURL string) (string, func(), error) {
	videoPath, _, cleanup, err := instagramYtDlpVideo(rawURL)
	return videoPath, cleanup, err
}

// instagramYtDlpVideo downloads an Instagram video with yt-dlp and returns its
// path and the post caption.
func instagramYtDlpVideo(rawURL string) (string, string, func(), error) {
	if instagramShortcode(rawURL) == "" {
		return "", "", func() {}, fmt.Errorf("Instagram reel shortcode not found")
	}

	cookiesArgs, cleanupCookies, err := instagramYtDlpCookiesArgs()
	if err != nil {
		return "", "", func() {}, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}
	defer cleanupCookies()

	tempDir, err := os.MkdirTemp("", "spotideezer-instagram-")
	if err != nil {
		return "", "", func() {}, fmt.Errorf("create Instagram temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(id)s.%(ext)s")

	args := append([]string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]", "-o", outputPattern}, ffmpegLocationArgs()...)
	args = append(args, "--merge-output-format", "mp4", "--write-info-json")
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	command := exec.Command("yt-dlp", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("run yt-dlp for Instagram video: %w: %s", err, output)
	}

	description := readInstagramDescription(tempDir)
	videoPath, err := findFileWithExtension(tempDir, ".mp4")
	if err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	videoPath, compressionCleanup, err := fitVideoToTelegramLimit(videoPath)
	if err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	return videoPath, description, func() {
		compressionCleanup()
		cleanup()
	}, nil
}

func instagramShortcode(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.Port() != "" {
		return ""
	}

	switch strings.ToLower(parsed.Hostname()) {
	case "instagram.com", "www.instagram.com", "m.instagram.com":
	default:
		return ""
	}

	match := instagramPostPathPattern.FindStringSubmatch(parsed.Path)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func findFileWithExtension(root, extension string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == extension {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan downloaded media: %w", err)
	}
	if found == "" {
		return "", fmt.Errorf("downloaded video file not found")
	}
	return found, nil
}
