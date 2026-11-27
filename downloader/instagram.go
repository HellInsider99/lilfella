package downloader

import (
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

// InstagramMedia downloads every image and video of an Instagram post, reel or
// carousel using gallery-dl. If gallery-dl fails, it falls back to yt-dlp,
// which can only fetch videos.
func InstagramMedia(rawURL string) ([]InstagramMediaItem, func(), error) {
	if instagramShortcode(rawURL) == "" {
		return nil, func() {}, fmt.Errorf("Instagram shortcode not found")
	}

	items, cleanup, galleryErr := instagramGalleryDLMedia(rawURL)
	if galleryErr == nil {
		return items, cleanup, nil
	}

	videoPath, videoCleanup, ytErr := InstagramReel(rawURL)
	if ytErr != nil {
		return nil, func() {}, fmt.Errorf("gallery-dl: %v; yt-dlp fallback: %w", galleryErr, ytErr)
	}
	return []InstagramMediaItem{{Path: videoPath, Type: "video"}}, videoCleanup, nil
}

func instagramGalleryDLMedia(rawURL string) ([]InstagramMediaItem, func(), error) {
	cookiesArgs, cleanupCookies, err := instagramYtDlpCookiesArgs()
	if err != nil {
		return nil, func() {}, fmt.Errorf("configure gallery-dl cookies: %w", err)
	}
	defer cleanupCookies()

	tempDir, err := os.MkdirTemp("", "lilfella-instagram-")
	if err != nil {
		return nil, func() {}, fmt.Errorf("create Instagram temp directory: %w", err)
	}
	cleanups := []func(){func() { _ = os.RemoveAll(tempDir) }}
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	// cookiesArgs is either empty or {"--cookies", path}; gallery-dl uses the same flag.
	args := []string{"--directory", tempDir, "--filename", "{num:>03}.{extension}"}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	command := exec.Command("gallery-dl", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return nil, func() {}, fmt.Errorf("run gallery-dl for Instagram post: %w: %s", err, output)
	}

	items, err := collectInstagramMedia(tempDir)
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}

	for i, item := range items {
		if item.Type != "video" {
			continue
		}
		fittedPath, compressionCleanup, err := fitVideoToTelegramLimit(item.Path)
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}
		cleanups = append(cleanups, compressionCleanup)
		items[i].Path = fittedPath
	}
	return items, cleanup, nil
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
	if instagramShortcode(rawURL) == "" {
		return "", func() {}, fmt.Errorf("Instagram reel shortcode not found")
	}

	cookiesArgs, cleanupCookies, err := instagramYtDlpCookiesArgs()
	if err != nil {
		return "", func() {}, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}
	defer cleanupCookies()

	tempDir, err := os.MkdirTemp("", "spotideezer-instagram-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create Instagram temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(id)s.%(ext)s")

	args := append([]string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]", "-o", outputPattern}, ffmpegLocationArgs()...)
	args = append(args, "--merge-output-format", "mp4")
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	command := exec.Command("yt-dlp", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("run yt-dlp for Instagram video: %w: %s", err, output)
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
