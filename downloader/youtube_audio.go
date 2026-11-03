package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func YouTubeAudio(rawURL string) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "spotideezer-youtube-audio-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create YouTube audio temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	outputPattern := filepath.Join(tempDir, "%(title)s.%(ext)s")

	args := append([]string{"-x", "--audio-format", "mp3", "-o", outputPattern}, ffmpegLocationArgs()...)
	args = append(args, rawURL)
	command := exec.Command("yt-dlp", args...)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("run yt-dlp for audio: %w: %s", err, output)
	}

	audioPath, err := findFileWithExtension(tempDir, ".mp3")
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := checkFileSize(audioPath); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return audioPath, cleanup, nil
}
