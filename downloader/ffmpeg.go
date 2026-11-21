package downloader

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	telegramFileSizeLimit       = 50_000_000
	telegramVideoTargetSize     = 49_000_000
	telegramVideoAudioBitrate   = 96_000
	telegramVideoMinimumBitrate = 64_000
	telegramVideoEncodeAttempts = 4
)

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
		return fmt.Errorf("file too large for Telegram: %.1f MB (limit: 50 MB)", float64(info.Size())/1_000_000)
	}
	return nil
}

func fitVideoToTelegramLimit(path string) (string, func(), error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", func() {}, fmt.Errorf("stat downloaded video: %w", err)
	}
	if info.Size() <= telegramVideoTargetSize {
		return path, func() {}, nil
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", func() {}, fmt.Errorf("video exceeds the safe Telegram upload target and ffmpeg is unavailable: %w", err)
	}
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return "", func() {}, fmt.Errorf("video exceeds the safe Telegram upload target and ffprobe is unavailable: %w", err)
	}

	durationOutput, err := exec.Command(ffprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		return "", func() {}, fmt.Errorf("read video duration with ffprobe: %w", err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(durationOutput)), 64)
	if err != nil || duration <= 0 || math.IsInf(duration, 0) || math.IsNaN(duration) {
		return "", func() {}, fmt.Errorf("invalid video duration reported by ffprobe: %q", strings.TrimSpace(string(durationOutput)))
	}
	videoBitrate, err := telegramVideoBitrate(duration)
	if err != nil {
		return "", func() {}, err
	}

	tempDir, err := os.MkdirTemp(filepath.Dir(path), "telegram-video-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create compressed video temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	compressedPath := filepath.Join(tempDir, "compressed.mp4")

	for attempt := 0; attempt < telegramVideoEncodeAttempts; attempt++ {
		command := exec.Command(ffmpegPath,
			"-hide_banner", "-loglevel", "error", "-y", "-i", path,
			"-c:v", "libx264", "-preset", "veryfast",
			"-b:v", strconv.Itoa(videoBitrate),
			"-maxrate", strconv.Itoa(videoBitrate),
			"-bufsize", strconv.Itoa(videoBitrate*2),
			"-c:a", "aac", "-b:a", strconv.Itoa(telegramVideoAudioBitrate),
			"-movflags", "+faststart", compressedPath)
		if output, err := command.CombinedOutput(); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("compress video for Telegram: %w: %s", err, strings.TrimSpace(string(output)))
		}

		compressedInfo, err := os.Stat(compressedPath)
		if err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("stat compressed video: %w", err)
		}
		if compressedInfo.Size() <= telegramVideoTargetSize {
			log.Printf("downloader: compressed video for Telegram: original_size_bytes=%d compressed_size_bytes=%d", info.Size(), compressedInfo.Size())
			return compressedPath, cleanup, nil
		}
		videoBitrate = int(float64(videoBitrate) * float64(telegramVideoTargetSize) / float64(compressedInfo.Size()) * 0.95)
		if videoBitrate < telegramVideoMinimumBitrate {
			break
		}
	}

	cleanup()
	return "", func() {}, fmt.Errorf("compressed video still exceeds the safe Telegram upload target after %d attempts", telegramVideoEncodeAttempts)
}

func VideoThumbnail(path string) (string, func(), error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", func() {}, fmt.Errorf("find ffmpeg for video thumbnail: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "telegram-video-thumb-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create video thumbnail temp directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	thumbnailPath := filepath.Join(tempDir, "thumbnail.jpg")

	for _, quality := range []string{"5", "10", "15"} {
		command := exec.Command(ffmpegPath,
			"-hide_banner", "-loglevel", "error", "-y",
			"-ss", "0.2", "-i", path,
			"-frames:v", "1",
			"-vf", "scale=320:320:force_original_aspect_ratio=decrease",
			"-q:v", quality, "-update", "1", thumbnailPath)
		if output, err := command.CombinedOutput(); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("generate video thumbnail: %w: %s", err, strings.TrimSpace(string(output)))
		}

		info, err := os.Stat(thumbnailPath)
		if err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("stat generated video thumbnail: %w", err)
		}
		if info.Size() <= 200_000 {
			return thumbnailPath, cleanup, nil
		}
	}

	cleanup()
	return "", func() {}, fmt.Errorf("generated video thumbnail exceeds Telegram's 200 KB limit")
}

func telegramVideoBitrate(duration float64) (int, error) {
	totalBitrate := float64(telegramVideoTargetSize) * 8 / duration
	videoBitrate := int((totalBitrate - telegramVideoAudioBitrate) * 0.97)
	if videoBitrate < telegramVideoMinimumBitrate {
		return 0, fmt.Errorf("video is too long to fit under Telegram's 50 MB limit at the minimum video bitrate")
	}
	return videoBitrate, nil
}
