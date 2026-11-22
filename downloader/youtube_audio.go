package downloader

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const telegramAudioThumbnailMaxSize = 200_000

func YouTubeAudio(rawURL string) (string, func(), error) {
	startedAt := time.Now()
	log.Printf("downloader: YouTube audio download started")
	cookiesArgs, cleanupCookies, err := ytDlpCookiesArgs()
	if err != nil {
		return "", func() {}, fmt.Errorf("configure yt-dlp cookies: %w", err)
	}
	defer cleanupCookies()
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

func YouTubeAudioThumbnail(videoID string) (string, func(), error) {
	client := &http.Client{Timeout: 15 * time.Second}
	thumbnailURL := "https://img.youtube.com/vi/" + videoID + "/mqdefault.jpg"
	response, err := client.Get(thumbnailURL)
	if err != nil {
		return "", func() {}, fmt.Errorf("request YouTube audio thumbnail: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", func() {}, fmt.Errorf("YouTube audio thumbnail returned %s", response.Status)
	}

	imageData, err := io.ReadAll(io.LimitReader(response.Body, telegramAudioThumbnailMaxSize+1))
	if err != nil {
		return "", func() {}, fmt.Errorf("read YouTube audio thumbnail: %w", err)
	}
	if len(imageData) > telegramAudioThumbnailMaxSize {
		return "", func() {}, fmt.Errorf("YouTube audio thumbnail exceeds Telegram's 200 KB limit")
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(imageData))
	if err != nil {
		return "", func() {}, fmt.Errorf("decode YouTube audio thumbnail: %w", err)
	}
	if config.Width > 320 || config.Height > 320 {
		return "", func() {}, fmt.Errorf("YouTube audio thumbnail dimensions exceed Telegram's 320x320 limit")
	}

	thumbnailFile, err := os.CreateTemp("", "telegram-audio-thumbnail-*.jpg")
	if err != nil {
		return "", func() {}, fmt.Errorf("create YouTube audio thumbnail file: %w", err)
	}
	thumbnailPath := thumbnailFile.Name()
	cleanup := func() { _ = os.Remove(thumbnailPath) }
	if _, err := thumbnailFile.Write(imageData); err != nil {
		_ = thumbnailFile.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write YouTube audio thumbnail: %w", err)
	}
	if err := thumbnailFile.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close YouTube audio thumbnail: %w", err)
	}
	return thumbnailPath, cleanup, nil
}
