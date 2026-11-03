package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const telegramFileSizeLimit = 50 * 1024 * 1024 // 50 MB

func ffmpegLocationArgs() []string {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err == nil {
		return []string{"--ffmpeg-location", filepath.Dir(ffmpegPath)}
	}
	homeDir, _ := os.UserHomeDir()
	candidates := []string{"/usr/local/bin", "/opt/homebrew/bin", "/usr/bin"}
	if homeDir != "" {
		candidates = append([]string{filepath.Join(homeDir, "bin")}, candidates...)
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "ffmpeg")); err == nil {
			return []string{"--ffmpeg-location", candidate}
		}
	}
	return nil
}

func checkFileSize(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat downloaded file: %w", err)
	}
	if info.Size() > telegramFileSizeLimit {
		return fmt.Errorf("file too large for Telegram: %d MB (limit: 50 MB)", info.Size()/(1024*1024))
	}
	return nil
}
