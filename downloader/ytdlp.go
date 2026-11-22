package downloader

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func ytDlpCookiesArgs() ([]string, func(), error) {
	noCleanup := func() {}
	cookiesPath := strings.TrimSpace(os.Getenv("YTDLP_COOKIES_FILE"))
	if cookiesPath == "" {
		return nil, noCleanup, nil
	}

	info, err := os.Stat(cookiesPath)
	if err != nil {
		return nil, noCleanup, fmt.Errorf("stat YTDLP_COOKIES_FILE: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, noCleanup, fmt.Errorf("YTDLP_COOKIES_FILE must point to a regular file")
	}

	source, err := os.Open(cookiesPath)
	if err != nil {
		return nil, noCleanup, fmt.Errorf("open YTDLP_COOKIES_FILE: %w", err)
	}
	defer source.Close()

	tempFile, err := os.CreateTemp("", "lilfella-yt-dlp-cookies-*.txt")
	if err != nil {
		return nil, noCleanup, fmt.Errorf("create writable yt-dlp cookie file: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := func() { _ = os.Remove(tempPath) }

	if _, err := io.Copy(tempFile, source); err != nil {
		tempFile.Close()
		cleanup()
		return nil, noCleanup, fmt.Errorf("copy YTDLP_COOKIES_FILE: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		cleanup()
		return nil, noCleanup, fmt.Errorf("close writable yt-dlp cookie file: %w", err)
	}

	return []string{"--cookies", tempPath}, cleanup, nil
}
