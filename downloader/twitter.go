package downloader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type twitterResponse struct {
	MediaURL  string          `json:"media_url"`
	MediaURLs json.RawMessage `json:"mediaURLs"`
}

type twitterMedia struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

func parseTwitterMediaURLs(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	// Try array of objects: [{"url":"...", "type":"..."}]
	var mediaObjects []twitterMedia
	if err := json.Unmarshal(raw, &mediaObjects); err == nil {
		urls := make([]string, 0, len(mediaObjects))
		for _, m := range mediaObjects {
			if m.URL != "" {
				urls = append(urls, m.URL)
			}
		}
		return urls
	}

	// Try array of strings: ["https://..."]
	var mediaStrings []string
	if err := json.Unmarshal(raw, &mediaStrings); err == nil {
		return mediaStrings
	}

	// Try single string: "https://..."
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && single != "" {
		return []string{single}
	}

	return nil
}

func TwitterVideoURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse Twitter URL: %w", err)
	}
	parsed.Scheme = "https"
	parsed.Host = "api.vxtwitter.com"

	response, err := http.Get(parsed.String())
	if err != nil {
		return "", fmt.Errorf("request Twitter proxy: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Twitter proxy returned %s", response.Status)
	}

	var payload twitterResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode Twitter proxy response: %w", err)
	}
	if payload.MediaURL != "" {
		return payload.MediaURL, nil
	}
	for _, mediaURL := range parseTwitterMediaURLs(payload.MediaURLs) {
		if strings.HasSuffix(strings.ToLower(mediaURL), ".mp4") || strings.Contains(mediaURL, "video") {
			return mediaURL, nil
		}
	}
	// If we have any media URL at all, return the first one
	if urls := parseTwitterMediaURLs(payload.MediaURLs); len(urls) > 0 {
		return urls[0], nil
	}
	return "", fmt.Errorf("Twitter video URL not found")
}
