package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MediaFile is a downloaded media file.
type MediaFile struct {
	Path string
	Type string // "image" or "video"
}

// runGalleryDL downloads rawURL with gallery-dl into a new temp directory,
// naming files by their position in the post ("001.jpg", "002.mp4", ...).
// cookiesArgs is either empty or {"--cookies", path}, as returned by the
// yt-dlp cookie helpers; gallery-dl accepts the same flag. The returned cleanup
// removes the directory.
func runGalleryDL(rawURL string, cookiesArgs []string, extraArgs ...string) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "lilfella-gallery-dl-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create gallery-dl temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	args := []string{"--directory", tempDir, "--filename", "{num:>03}.{extension}"}
	args = append(args, extraArgs...)
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	if output, err := exec.Command("gallery-dl", args...).CombinedOutput(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("run gallery-dl: %w: %s", err, output)
	}
	return tempDir, cleanup, nil
}

// collectMediaFiles returns downloaded media files in dir, sorted by name
// (gallery-dl numbers them in post order), classified as image or video.
func collectMediaFiles(dir string) ([]MediaFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("scan downloaded media: %w", err)
	}

	var items []MediaFile
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
		items = append(items, MediaFile{Path: filepath.Join(dir, entry.Name()), Type: mediaType})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no downloaded media found")
	}
	return items, nil
}
