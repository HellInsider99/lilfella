package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var instagramShortcodePattern = regexp.MustCompile(`instagram\.com/(?:reel|p)/([A-Za-z0-9_-]+)`)

func InstagramReel(rawURL string) (string, func(), error) {
	match := instagramShortcodePattern.FindStringSubmatch(rawURL)
	if len(match) < 2 {
		return "", func() {}, fmt.Errorf("Instagram reel shortcode not found")
	}

	tempDir, err := os.MkdirTemp("", "spotideezer-instagram-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create Instagram temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	command := exec.Command("python3", "-m", "instaloader", "--dirname-pattern="+tempDir, "--", "-"+match[1])
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("run instaloader: %w: %s", err, output)
	}

	videoPath, err := findFileWithExtension(tempDir, ".mp4")
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	return videoPath, cleanup, nil
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
		return "", fmt.Errorf("Instagram video file not found")
	}
	return found, nil
}
