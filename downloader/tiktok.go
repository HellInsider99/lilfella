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

var tikTokVideoPathPattern = regexp.MustCompile(`^/@[^/]+/video/[0-9]+/?$`)

func TikTokVideo(rawURL string) (string, func(), error) {
	if err := validateTikTokURL(rawURL); err != nil {
		return "", func() {}, err
	}

	tempDir, err := os.MkdirTemp("", "spotideezer-tiktok-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create TikTok temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(id)s.%(ext)s")

	command := exec.Command("yt-dlp", "-f", "best[ext=mp4]/best", "-o", outputPattern, rawURL)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("run yt-dlp for TikTok video: %w: %s", err, output)
	}

	videoPath, err := findFileWithExtension(tempDir, ".mp4")
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	return videoPath, cleanup, nil
}

func validateTikTokURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse TikTok URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("unsupported TikTok URL scheme")
	}
	if parsed.User != nil || parsed.Port() != "" {
		return fmt.Errorf("invalid TikTok URL authority")
	}

	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "tiktok.com", "www.tiktok.com", "m.tiktok.com":
		if !tikTokVideoPathPattern.MatchString(parsed.Path) {
			return fmt.Errorf("TikTok video URL path not found")
		}
	case "vm.tiktok.com", "vt.tiktok.com":
		if strings.Trim(parsed.Path, "/") == "" {
			return fmt.Errorf("TikTok short-link code not found")
		}
	default:
		return fmt.Errorf("unsupported TikTok URL host")
	}
	return nil
}
