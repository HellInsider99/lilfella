package downloader

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func YouTubeAudio(rawURL string) (string, func(), error) {
	startedAt := time.Now()
	log.Printf("downloader: YouTube audio download started")
	cookiesArgs, err := ytDlpCookiesArgs()
	if err != nil {
		return "", func() {}, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}
	tempDir, err := os.MkdirTemp("", "spotideezer-youtube-audio-")
	if err != nil {
		log.Printf("downloader: create YouTube audio temp directory failed: %v", err)
		return "", func() {}, fmt.Errorf("create YouTube audio temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(title)s.%(ext)s")

	args := append([]string{"-x", "--audio-format", "mp3", "-o", outputPattern}, ffmpegLocationArgs()...)
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	command := exec.Command("yt-dlp", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		log.Printf("downloader: yt-dlp audio command failed after %s: %v; output: %s", time.Since(startedAt).Round(time.Millisecond), err, output)
		return "", func() {}, fmt.Errorf("run yt-dlp for audio: %w: %s", err, output)
	}

	audioPath, err := findFileWithExtension(tempDir, ".mp3")
	if err != nil {
		cleanup()
		log.Printf("downloader: YouTube audio file lookup failed after %s: %v", time.Since(startedAt).Round(time.Millisecond), err)
		return "", func() {}, err
	}
	if err := checkFileSize(audioPath); err != nil {
		cleanup()
		log.Printf("downloader: YouTube audio file validation failed after %s: %v", time.Since(startedAt).Round(time.Millisecond), err)
		return "", func() {}, err
	}
	info, err := os.Stat(audioPath)
	if err != nil {
		cleanup()
		log.Printf("downloader: stat YouTube audio file failed: %v", err)
		return "", func() {}, fmt.Errorf("stat downloaded audio file: %w", err)
	}
	log.Printf("downloader: YouTube audio download succeeded after %s: size_bytes=%d", time.Since(startedAt).Round(time.Millisecond), info.Size())
	return audioPath, cleanup, nil
}
