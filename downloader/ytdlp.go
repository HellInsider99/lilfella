package downloader

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func ytDlpCookiesArgs() ([]string, func(), error) {
	return ytDlpCookiesArgsFromEnv("YTDLP_COOKIES_FILE")
}

func instagramYtDlpCookiesArgs() ([]string, func(), error) {
	if strings.TrimSpace(os.Getenv("INSTAGRAM_COOKIES_FILE")) == "" {
		return ytDlpCookiesArgs()
	}
	return ytDlpCookiesArgsFromEnv("INSTAGRAM_COOKIES_FILE")
}

func ytDlpCookiesArgsFromEnv(envName string) ([]string, func(), error) {
	noCleanup := func() {}
	cookiesPath := strings.TrimSpace(os.Getenv(envName))
	if cookiesPath == "" {
		return nil, noCleanup, nil
	}

	info, err := os.Stat(cookiesPath)
	if err != nil {
		return nil, noCleanup, fmt.Errorf("stat %s: %w", envName, err)
	}
	if !info.Mode().IsRegular() {
		return nil, noCleanup, fmt.Errorf("%s must point to a regular file", envName)
	}

	source, err := os.Open(cookiesPath)
	if err != nil {
		return nil, noCleanup, fmt.Errorf("open %s: %w", envName, err)
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
		return nil, noCleanup, fmt.Errorf("copy %s: %w", envName, err)
	}
	if err := tempFile.Close(); err != nil {
		cleanup()
		return nil, noCleanup, fmt.Errorf("close writable yt-dlp cookie file: %w", err)
	}

	return []string{"--cookies", tempPath}, cleanup, nil
}
