package downloader

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// YouTubeVideo downloads a YouTube video (Shorts) with yt-dlp and returns its
// path and title ("" if unavailable).
func YouTubeVideo(rawURL string) (string, string, func(), error) {
	startedAt := time.Now()
	log.Printf("downloader: YouTube video download started")
	cookiesArgs, cleanupCookies, err := ytDlpCookiesArgs()
	if err != nil {
		return "", "", func() {}, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}
	defer cleanupCookies()
	tempDir, err := os.MkdirTemp("", "spotideezer-youtube-video-")
	if err != nil {
		log.Printf("downloader: create YouTube video temp directory failed: %v", err)
		return "", "", func() {}, fmt.Errorf("create YouTube temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(id)s.%(ext)s")

	args := append([]string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best", "-o", outputPattern, "--write-info-json"}, ffmpegLocationArgs()...)
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	command := exec.Command("yt-dlp", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		log.Printf("downloader: yt-dlp video command failed after %s: %v; output: %s", time.Since(startedAt).Round(time.Millisecond), err, output)
		return "", "", func() {}, fmt.Errorf("run yt-dlp for video: %w: %s", err, output)
	}

	// A Short's title is its visible text; the description is usually tags and links.
	title := readMetadataText(tempDir, "title")
	videoPath, err := findFileWithExtension(tempDir, ".mp4")
	if err != nil {
		cleanup()
		log.Printf("downloader: YouTube video file lookup failed after %s: %v", time.Since(startedAt).Round(time.Millisecond), err)
		return "", "", func() {}, err
	}
	videoPath, compressionCleanup, err := fitVideoToTelegramLimit(videoPath)
	if err != nil {
		cleanup()
		log.Printf("downloader: YouTube video file validation failed after %s: %v", time.Since(startedAt).Round(time.Millisecond), err)
		return "", "", func() {}, err
	}
	info, err := os.Stat(videoPath)
	if err != nil {
		compressionCleanup()
		cleanup()
		log.Printf("downloader: stat YouTube video file failed: %v", err)
		return "", "", func() {}, fmt.Errorf("stat downloaded video file: %w", err)
	}
	log.Printf("downloader: YouTube video download succeeded after %s: size_bytes=%d", time.Since(startedAt).Round(time.Millisecond), info.Size())
	return videoPath, title, func() {
		compressionCleanup()
		cleanup()
	}, nil
}
