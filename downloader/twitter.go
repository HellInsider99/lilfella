package downloader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

type twitterResponse struct {
	Text          string             `json:"text"`
	MediaURL      string             `json:"media_url"`
	MediaURLs     json.RawMessage    `json:"mediaURLs"`
	MediaExtended []TwitterMediaItem `json:"media_extended"`
}

type TwitterMediaItem struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

func parseTwitterMediaURLs(raw json.RawMessage) []TwitterMediaItem {
	if len(raw) == 0 {
		return nil
	}

	// Try array of objects: [{"url":"...", "type":"..."}]
	var mediaObjects []TwitterMediaItem
	if err := json.Unmarshal(raw, &mediaObjects); err == nil {
		media := make([]TwitterMediaItem, 0, len(mediaObjects))
		for _, item := range mediaObjects {
			if item.URL != "" {
				if item.Type == "" {
					item.Type = inferTwitterMediaType(item.URL)
				}
				media = append(media, item)
			}
		}
		return media
	}

	// Try array of strings: ["https://..."]
	var mediaStrings []string
	if err := json.Unmarshal(raw, &mediaStrings); err == nil {
		media := make([]TwitterMediaItem, 0, len(mediaStrings))
		for _, mediaURL := range mediaStrings {
			if mediaURL != "" {
				media = append(media, TwitterMediaItem{URL: mediaURL, Type: inferTwitterMediaType(mediaURL)})
			}
		}
		return media
	}

	// Try single string: "https://..."
	var single string
	if err := json.Unmarshal(raw, &single); err == nil && single != "" {
		return []TwitterMediaItem{{URL: single, Type: inferTwitterMediaType(single)}}
	}

	return nil
}

// TwitterMedia returns a tweet's media and its text ("" if none).
func TwitterMedia(rawURL string) ([]TwitterMediaItem, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("parse Twitter URL: %w", err)
	}
	parsed.Scheme = "https"
	parsed.Host = "api.vxtwitter.com"

	response, err := http.Get(parsed.String())
	if err != nil {
		return nil, "", fmt.Errorf("request Twitter proxy: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Twitter proxy returned %s", response.Status)
	}

	var payload twitterResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, "", fmt.Errorf("decode Twitter proxy response: %w", err)
	}
	media := parseTwitterMedia(payload)
	if len(media) == 0 {
		return nil, "", fmt.Errorf("Twitter media URL not found")
	}
	return media, cleanTweetText(payload.Text), nil
}

var trailingTCoLinkPattern = regexp.MustCompile(`(?:\s*https://t\.co/[A-Za-z0-9]+)+\s*$`)

// cleanTweetText removes the t.co links at the end of a tweet's text. Twitter
// appends them for attached media, which the bot sends anyway.
func cleanTweetText(text string) string {
	return strings.TrimSpace(trailingTCoLinkPattern.ReplaceAllString(text, ""))
}

func parseTwitterMedia(payload twitterResponse) []TwitterMediaItem {
	if len(payload.MediaExtended) > 0 {
		media := make([]TwitterMediaItem, 0, len(payload.MediaExtended))
		for _, item := range payload.MediaExtended {
			if item.URL != "" {
				if item.Type == "" {
					item.Type = inferTwitterMediaType(item.URL)
				}
				media = append(media, item)
			}
		}
		if len(media) > 0 {
			return media
		}
	}

	media := parseTwitterMediaURLs(payload.MediaURLs)
	if len(media) == 0 && payload.MediaURL != "" {
		media = []TwitterMediaItem{{URL: payload.MediaURL, Type: inferTwitterMediaType(payload.MediaURL)}}
	}
	return media
}

func TwitterVideoURL(rawURL string) (string, error) {
	media, _, err := TwitterMedia(rawURL)
	if err != nil {
		return "", err
	}
	for _, item := range media {
		if item.Type == "video" || item.Type == "animated_gif" {
			return item.URL, nil
		}
	}
	for _, item := range media {
		if item.Type != "image" {
			return item.URL, nil
		}
	}
	return "", fmt.Errorf("Twitter video URL not found")
}

func inferTwitterMediaType(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if strings.EqualFold(filepath.Ext(parsed.Path), ".mp4") || strings.EqualFold(parsed.Hostname(), "video.twimg.com") {
		return "video"
	}
	if strings.EqualFold(parsed.Hostname(), "pbs.twimg.com") || strings.EqualFold(filepath.Ext(parsed.Path), ".jpg") ||
		strings.EqualFold(filepath.Ext(parsed.Path), ".jpeg") || strings.EqualFold(filepath.Ext(parsed.Path), ".png") {
		return "image"
	}
	return ""
}
