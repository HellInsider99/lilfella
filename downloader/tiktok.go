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

var (
	tikTokVideoPathPattern = regexp.MustCompile(`^/@[^/]+/(?:video|photo)/[0-9]+/?$`)
	tikTokPhotoPathPattern = regexp.MustCompile(`^/@[^/]+/photo/[0-9]+/?$`)
	tikTokShortPathPattern = regexp.MustCompile(`^/t/[^/]+/?$`)
)

// TikTokVideo downloads a TikTok video with yt-dlp and returns its path and
// the post caption ("" if it has none).
func TikTokVideo(rawURL string) (string, string, func(), error) {
	if err := validateTikTokURL(rawURL); err != nil {
		return "", "", func() {}, err
	}

	cookiesArgs, cleanupCookies, err := ytDlpCookiesArgs()
	if err != nil {
		return "", "", func() {}, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}
	defer cleanupCookies()

	tempDir, err := os.MkdirTemp("", "spotideezer-tiktok-")
	if err != nil {
		return "", "", func() {}, fmt.Errorf("create TikTok temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(id)s.%(ext)s")

	args := append([]string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]", "-o", outputPattern, "--write-info-json"}, ffmpegLocationArgs()...)
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	command := exec.Command("yt-dlp", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("run yt-dlp for TikTok video: %w: %s", err, output)
	}

	description := readMetadataText(tempDir, "description")
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

// IsTikTokPhotoURL reports whether rawURL is a full TikTok photo-post link.
// Short links (vm.tiktok.com, /t/...) can point to either kind and return false.
func IsTikTokPhotoURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && tikTokPhotoPathPattern.MatchString(parsed.Path)
}

// TikTokPhotos downloads the pictures of a TikTok photo post (slideshow) in
// post order with gallery-dl; yt-dlp cannot download them. The slideshow's
// background music is skipped. It also returns the post caption ("" if none).
func TikTokPhotos(rawURL string) ([]MediaFile, string, func(), error) {
	if err := validateTikTokURL(rawURL); err != nil {
		return nil, "", func() {}, err
	}

	cookiesArgs, cleanupCookies, err := ytDlpCookiesArgs()
	if err != nil {
		return nil, "", func() {}, fmt.Errorf("configure gallery-dl cookies: %w", err)
	}
	defer cleanupCookies()

	tempDir, cleanup, err := runGalleryDL(rawURL, cookiesArgs, "-o", "audio=false", "-o", "videos=false", "--write-metadata")
	if err != nil {
		return nil, "", func() {}, fmt.Errorf("TikTok photo post: %w", err)
	}

	files, err := collectMediaFiles(tempDir)
	if err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	// gallery-dl keeps TikTok's own field name, "desc", for the caption.
	description := readMetadataText(tempDir, "desc", "description")
	var images []MediaFile
	for _, file := range files {
		if file.Type == "image" {
			images = append(images, file)
		}
	}
	if len(images) == 0 {
		cleanup()
		return nil, "", func() {}, fmt.Errorf("no TikTok photos found")
	}
	return images, description, cleanup, nil
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
		if !tikTokVideoPathPattern.MatchString(parsed.Path) && !tikTokShortPathPattern.MatchString(parsed.Path) {
			return fmt.Errorf("TikTok video or photo URL path not found")
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
