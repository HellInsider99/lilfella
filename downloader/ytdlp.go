package downloader

import (
	"fmt"
	"os"
	"strings"
)

func ytDlpCookiesArgs() ([]string, error) {
	cookiesPath := strings.TrimSpace(os.Getenv("YTDLP_COOKIES_FILE"))
	if cookiesPath == "" {
		return nil, nil
	}

	info, err := os.Stat(cookiesPath)
	if err != nil {
		return nil, fmt.Errorf("stat YTDLP_COOKIES_FILE: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("YTDLP_COOKIES_FILE must point to a regular file")
	}

	return []string{"--cookies", cookiesPath}, nil
}
