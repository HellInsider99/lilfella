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

func InstagramReel(rawURL string) (string, func(), error) {
	if instagramShortcode(rawURL) == "" {
		return "", func() {}, fmt.Errorf("Instagram reel shortcode not found")
	}

	cookiesArgs, err := ytDlpCookiesArgs()
	if err != nil {
		return "", func() {}, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}

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
