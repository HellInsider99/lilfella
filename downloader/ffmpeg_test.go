package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFitVideoToTelegramLimitLeavesSmallFileUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.mp4")
	if err := os.WriteFile(path, []byte("video"), 0600); err != nil {
		t.Fatal(err)
	}

	got, cleanup, err := fitVideoToTelegramLimit(path)
	if err != nil {
		t.Fatalf("fitVideoToTelegramLimit() error = %v", err)
	}
	defer cleanup()
	if got != path {
		t.Errorf("fitVideoToTelegramLimit() path = %q, want %q", got, path)
	}
}

func TestTelegramVideoBitrate(t *testing.T) {
	bitrate, err := telegramVideoBitrate(60)
	if err != nil {
		t.Fatalf("telegramVideoBitrate(60) error = %v", err)
	}
	if bitrate < telegramVideoMinimumBitrate {
		t.Errorf("telegramVideoBitrate(60) = %d, want at least %d", bitrate, telegramVideoMinimumBitrate)
	}

	if _, err := telegramVideoBitrate(7_000); err == nil {
		t.Fatal("telegramVideoBitrate(7000) error = nil, want too-long-video error")
	}
}
