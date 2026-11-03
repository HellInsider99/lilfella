package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func YouTubeVideo(rawURL string) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "spotideezer-youtube-video-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create YouTube temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(id)s.%(ext)s")

	args := append([]string{"-f", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best", "-o", outputPattern}, ffmpegLocationArgs()...)
	args = append(args, rawURL)
	command := exec.Command("yt-dlp", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("run yt-dlp for video: %w: %s", err, output)
	}

	videoPath, err := findFileWithExtension(tempDir, ".mp4")
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := checkFileSize(videoPath); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return videoPath, cleanup, nil
}
