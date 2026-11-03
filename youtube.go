package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type YouTubePlatform struct {
	apiKey     string
	client     *http.Client
	urlPattern *regexp.Regexp
}

type youtubeSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title       string `json:"title"`
			ChannelName string `json:"channelTitle"`
		} `json:"snippet"`
	} `json:"items"`
}

type youtubeVideoResponse struct {
	Items []struct {
		Snippet struct {
			Title        string `json:"title"`
			ChannelTitle string `json:"channelTitle"`
			Description  string `json:"description"`
		} `json:"snippet"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
	} `json:"items"`
}

type youtubeOEmbedResponse struct {
	Title      string `json:"title"`
	AuthorName string `json:"author_name"`
}

var youtubeTrackDelimiterPattern = regexp.MustCompile(`\s+[-·]\s+`)
var youtubeTopicSuffixPattern = regexp.MustCompile(`(?i)\s*-\s*topic\s*$`)
var youtubeReleasePrefixPattern = regexp.MustCompile(`(?i)^release\s*-\s*`)

func parseYouTubeTrackName(rawTitle, fallbackArtist string) (string, string) {
	parts := youtubeTrackDelimiterPattern.Split(strings.TrimSpace(rawTitle), 2)
	if len(parts) != 2 {
		return rawTitle, fallbackArtist
	}

	leftTitle := strings.TrimSpace(parts[0])
	rightTitle := strings.TrimSpace(parts[1])
	if strings.TrimSpace(fallbackArtist) == "" {
		return leftTitle, rightTitle
	}

	cleanFallbackArtist := CleanArtistName(fallbackArtist)
	leftArtistSimilarity := CalculateSimilarity(cleanFallbackArtist, CleanArtistName(leftTitle))
	rightArtistSimilarity := CalculateSimilarity(cleanFallbackArtist, CleanArtistName(rightTitle))

	if leftArtistSimilarity > 80.0 && leftArtistSimilarity >= rightArtistSimilarity {
		return rightTitle, fallbackArtist
	}
	if rightArtistSimilarity > 80.0 {
		return leftTitle, fallbackArtist
	}

	return leftTitle, fallbackArtist
}

func NewYouTubePlatform(apiKey string) *YouTubePlatform {
	return &YouTubePlatform{
		apiKey:     apiKey,
		client:     http.DefaultClient,
		urlPattern: regexp.MustCompile(`https?://(?:www\.)?(?:youtube\.com/watch\?v=[^\s&]+|youtu\.be/[^\s?]+|music\.youtube\.com/watch\?v=[^\s&]+)`),
	}
}

func (platform *YouTubePlatform) Name() string {
	return "YouTube Music"
}

func (platform *YouTubePlatform) ExtractURL(text string) (string, bool) {
	match := platform.urlPattern.FindString(text)
	return strings.TrimRight(match, ".,!?)]}"), match != ""
}

func (platform *YouTubePlatform) TrackFromURL(trackURL string) (Track, error) {
	if platform.apiKey == "" {
		return Track{}, fmt.Errorf("YouTube API key is not set")
	}
	videoID, err := extractYouTubeVideoID(trackURL)
	if err != nil {
		return Track{}, err
	}

	track, primaryErr := platform.videoTrack(videoID)
	if primaryErr == nil && strings.TrimSpace(track.Title) != "" && (!youtubeArtistLooksSuspicious(track.Artist) || youtubeArtistIsTopicChannel(track.Artist)) {
		track.Artist = cleanYouTubeArtistName(track.Artist)
		return track, nil
	}
	if primaryErr != nil {
		log.Printf("YouTube primary metadata failed, trying oEmbed fallback: %v", primaryErr)
	} else {
		log.Printf("YouTube primary metadata looks suspicious (title=%q, artist=%q), trying oEmbed fallback", track.Title, track.Artist)
	}

	fallbackTrack, fallbackErr := platform.trackFromOEmbed(trackURL, videoID)
	if fallbackErr != nil {
		return Track{}, fmt.Errorf("YouTube metadata failed: primary=%v; oEmbed fallback=%w", primaryErr, fallbackErr)
	}
	return fallbackTrack, nil
}

func youtubeArtistLooksSuspicious(artist string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(artist))
	if cleaned == "" {
		return true
	}
	if youtubeArtistIsTopicChannel(cleaned) {
		return false
	}
	for _, noise := range []string{"release", "provided to", "provided by", "official audio", "youtube"} {
		if strings.Contains(cleaned, noise) {
			return true
		}
	}
	return false
}

func youtubeArtistIsTopicChannel(artist string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(artist))
	return strings.HasSuffix(cleaned, "- topic") || strings.Contains(cleaned, "release - topic")
}

func cleanYouTubeArtistName(artist string) string {
	cleaned := strings.TrimSpace(artist)
	cleaned = youtubeTopicSuffixPattern.ReplaceAllString(cleaned, "")
	cleaned = youtubeReleasePrefixPattern.ReplaceAllString(cleaned, "")
	return strings.TrimSpace(cleaned)
}

func (platform *YouTubePlatform) trackFromOEmbed(trackURL, videoID string) (Track, error) {
	oEmbedURL := "https://www.youtube.com/oembed?" + url.Values{
		"url":    {trackURL},
		"format": {"json"},
	}.Encode()
	response, err := platform.client.Get(oEmbedURL)
	if err != nil {
		return Track{}, fmt.Errorf("request oEmbed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Track{}, fmt.Errorf("oEmbed returned %s", response.Status)
	}

	var metadata youtubeOEmbedResponse
	if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
		return Track{}, fmt.Errorf("decode oEmbed response: %w", err)
	}
	if strings.TrimSpace(metadata.Title) == "" || (youtubeArtistLooksSuspicious(metadata.AuthorName) && !youtubeArtistIsTopicChannel(metadata.AuthorName)) {
		return Track{}, fmt.Errorf("oEmbed metadata is incomplete or suspicious: title=%q, artist=%q", metadata.Title, metadata.AuthorName)
	}
	duration, err := platform.videoDuration(videoID)
	if err != nil {
		return Track{}, fmt.Errorf("get duration for oEmbed fallback: %w", err)
	}
	return Track{Title: metadata.Title, Artist: cleanYouTubeArtistName(metadata.AuthorName), DurationMs: duration}, nil
}

func extractYouTubeVideoID(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid YouTube URL: %w", err)
	}
	host := strings.ToLower(strings.TrimPrefix(parsedURL.Host, "www."))
	if host == "youtu.be" {
		if videoID := strings.Trim(parsedURL.Path, "/"); videoID != "" {
			return videoID, nil
		}
	}
	if host == "youtube.com" || host == "music.youtube.com" {
		if videoID := parsedURL.Query().Get("v"); videoID != "" {
			return videoID, nil
		}
	}
	return "", fmt.Errorf("YouTube video ID not found in URL")
}

func (platform *YouTubePlatform) videoTrack(videoID string) (Track, error) {
	params := url.Values{
		"part": {"snippet,contentDetails"},
		"id":   {videoID},
		"key":  {platform.apiKey},
	}
	response, err := platform.client.Get("https://www.googleapis.com/youtube/v3/videos?" + params.Encode())
	if err != nil {
		return Track{}, fmt.Errorf("YouTube video API request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Track{}, fmt.Errorf("YouTube video API returned %s", response.Status)
	}
	var videoResponse youtubeVideoResponse
	if err := json.NewDecoder(response.Body).Decode(&videoResponse); err != nil {
		return Track{}, fmt.Errorf("decode YouTube video API response: %w", err)
	}
	if len(videoResponse.Items) == 0 {
		return Track{}, fmt.Errorf("YouTube video not found")
	}
	item := videoResponse.Items[0]
	duration, err := parseYouTubeDuration(item.ContentDetails.Duration)
	if err != nil {
		return Track{}, fmt.Errorf("parse YouTube duration: %w", err)
	}
	artist := item.Snippet.ChannelTitle
	if strings.TrimSpace(artist) == "" {
		artist = strings.TrimSpace(strings.SplitN(item.Snippet.Description, "\n", 2)[0])
	}
	if strings.TrimSpace(item.Snippet.Title) == "" || artist == "" {
		return Track{}, fmt.Errorf("YouTube video metadata is incomplete")
	}
	return Track{Title: item.Snippet.Title, Artist: artist, DurationMs: duration}, nil
}

func (platform *YouTubePlatform) SearchTrack(track Track) (string, error) {
	if platform.apiKey == "" {
		return "", fmt.Errorf("YouTube API key is not set")
	}

	params := url.Values{
		"part":       {"snippet"},
		"type":       {"video"},
		"maxResults": {"10"},
		"q":          {strings.TrimSpace(track.Artist + " " + track.Title)},
		"key":        {platform.apiKey},
	}
	response, err := platform.client.Get("https://www.googleapis.com/youtube/v3/search?" + params.Encode())
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("YouTube search API returned %s", response.Status)
	}

	var searchResponse youtubeSearchResponse
	if err := json.NewDecoder(response.Body).Decode(&searchResponse); err != nil {
		return "", err
	}
	results := make([]TrackResult, 0, len(searchResponse.Items))
	for _, item := range searchResponse.Items {
		if item.ID.VideoID == "" {
			continue
		}
		duration, durationErr := platform.videoDuration(item.ID.VideoID)
		if durationErr != nil {
			continue
		}
		results = append(results, TrackResult{
			Title:      item.Snippet.Title,
			Artist:     item.Snippet.ChannelName,
			DurationMs: duration,
			URL:        "https://music.youtube.com/watch?v=" + url.QueryEscape(item.ID.VideoID),
		})
	}

	match := FindBestYouTubeMatch(track.DurationMs, CleanTrackTitle(track.Title), results)
	if match == nil {
		return "", fmt.Errorf("YouTube track not found")
	}
	return match.URL, nil
}

func (platform *YouTubePlatform) videoDuration(videoID string) (int, error) {
	params := url.Values{
		"part": {"contentDetails"},
		"id":   {videoID},
		"key":  {platform.apiKey},
	}
	response, err := platform.client.Get("https://www.googleapis.com/youtube/v3/videos?" + params.Encode())
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("YouTube video API returned %s", response.Status)
	}

	var videoResponse youtubeVideoResponse
	if err := json.NewDecoder(response.Body).Decode(&videoResponse); err != nil {
		return 0, err
	}
	if len(videoResponse.Items) == 0 {
		return 0, fmt.Errorf("YouTube video not found")
	}
	return parseYouTubeDuration(videoResponse.Items[0].ContentDetails.Duration)
}

func parseYouTubeDuration(duration string) (int, error) {
	match := regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`).FindStringSubmatch(duration)
	if len(match) == 0 {
		return 0, fmt.Errorf("invalid YouTube duration %q", duration)
	}

	values := make([]int, 3)
	for index := 1; index <= 3; index++ {
		if match[index] == "" {
			continue
		}
		value, err := strconv.Atoi(match[index])
		if err != nil {
			return 0, err
		}
		values[index-1] = value
	}
	return (values[0]*3600 + values[1]*60 + values[2]) * 1000, nil
}

func SearchYouTubeMusic(apiKey, artist, track string) (string, error) {
	return SearchYouTubeMusicWithClient(http.DefaultClient, apiKey, artist, track)
}

func SearchYouTubeMusicWithClient(client *http.Client, apiKey, artist, track string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("YouTube API key is not set")
	}

	params := url.Values{
		"part":       {"snippet"},
		"type":       {"video"},
		"maxResults": {"1"},
		"q":          {strings.TrimSpace(artist + " " + track)},
		"key":        {apiKey},
	}
	response, err := client.Get("https://www.googleapis.com/youtube/v3/search?" + params.Encode())
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("YouTube search API returned %s", response.Status)
	}

	var searchResponse youtubeSearchResponse
	if err := json.NewDecoder(response.Body).Decode(&searchResponse); err != nil {
		return "", err
	}
	if len(searchResponse.Items) == 0 || searchResponse.Items[0].ID.VideoID == "" {
		return "", fmt.Errorf("YouTube track not found")
	}

	return "https://music.youtube.com/watch?v=" + url.QueryEscape(searchResponse.Items[0].ID.VideoID), nil
}
