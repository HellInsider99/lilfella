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

type AppleMusicPlatform struct {
	client     *http.Client
	urlPattern *regexp.Regexp
}

const appleMusicDurationToleranceMs = 15_000

type appleMusicResponse struct {
	Results []struct {
		TrackName       string `json:"trackName"`
		ArtistName      string `json:"artistName"`
		TrackTimeMillis int    `json:"trackTimeMillis"`
		TrackViewURL    string `json:"trackViewUrl"`
	} `json:"results"`
}

func NewAppleMusicPlatform() *AppleMusicPlatform {
	return &AppleMusicPlatform{
		client:     http.DefaultClient,
		urlPattern: regexp.MustCompile(`https?://music\.apple\.com/[^\s]+`),
	}
}

func (platform *AppleMusicPlatform) Name() string {
	return "Apple Music"
}

func (platform *AppleMusicPlatform) ExtractURL(text string) (string, bool) {
	match := platform.urlPattern.FindString(text)
	return strings.TrimRight(match, ".,!?)]}"), match != ""
}

func (platform *AppleMusicPlatform) TrackFromURL(trackURL string) (Track, error) {
	trackID, err := appleMusicTrackID(trackURL)
	if err != nil {
		return Track{}, err
	}

	response, err := platform.client.Get("https://itunes.apple.com/lookup?id=" + url.QueryEscape(trackID))
	if err != nil {
		return Track{}, fmt.Errorf("Apple Music lookup request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Track{}, fmt.Errorf("Apple Music lookup returned %s", response.Status)
	}

	var lookup appleMusicResponse
	if err := json.NewDecoder(response.Body).Decode(&lookup); err != nil {
		return Track{}, fmt.Errorf("decode Apple Music lookup response: %w", err)
	}
	if len(lookup.Results) == 0 || lookup.Results[0].TrackName == "" || lookup.Results[0].ArtistName == "" {
		return Track{}, fmt.Errorf("Apple Music track not found")
	}

	result := lookup.Results[0]
	return Track{Title: result.TrackName, Artist: result.ArtistName, DurationMs: result.TrackTimeMillis}, nil
}

func (platform *AppleMusicPlatform) SearchTrack(track Track) (string, error) {
	results, err := platform.searchResults(strings.TrimSpace(track.Artist + " " + track.Title))
	if err != nil {
		return "", err
	}

	match := findBestAppleMusicMatchForSource(track.DurationMs, CleanTrackTitle(track.Title), CleanArtistName(track.Artist), track.SourcePlatform, results)
	if match != nil {
		return match.URL, nil
	}

	if strings.TrimSpace(track.Artist) != "" {
		results, err = platform.searchResults(strings.TrimSpace(track.Title))
		if err != nil {
			return "", err
		}
		match = findBestAppleMusicMatchForSource(track.DurationMs, CleanTrackTitle(track.Title), CleanArtistName(track.Artist), track.SourcePlatform, results)
		if match != nil {
			return match.URL, nil
		}
	}

	return "", fmt.Errorf("Apple Music track not found")
}

func (platform *AppleMusicPlatform) searchResults(term string) ([]TrackResult, error) {
	query := url.Values{"term": {term}, "entity": {"song"}, "limit": {"25"}, "country": {"UA"}, "explicit": {"Yes"}}.Encode()
	response, err := platform.client.Get("https://itunes.apple.com/search?" + query)
	if err != nil {
		return nil, fmt.Errorf("Apple Music search request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Apple Music search returned %s", response.Status)
	}
	var searchResponse appleMusicResponse
	if err := json.NewDecoder(response.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("decode Apple Music search response: %w", err)
	}
	results := make([]TrackResult, 0, len(searchResponse.Results))
	for _, item := range searchResponse.Results {
		if item.TrackName == "" || item.ArtistName == "" {
			continue
		}
		results = append(results, TrackResult{Title: item.TrackName, Artist: item.ArtistName, DurationMs: item.TrackTimeMillis, URL: item.TrackViewURL})
	}
	return results, nil
}

func findBestAppleMusicMatch(originalDurationMs int, originalCleanTitle, originalCleanArtist string, results []TrackResult) *TrackResult {
	originalArtists := cleanCollaboratingArtists(originalCleanArtist)
	for index := range results {
		result := &results[index]
		durationDifference := originalDurationMs - result.DurationMs
		if durationDifference < 0 {
			durationDifference = -durationDifference
		}
		cleanResultTitle := CleanTrackTitle(result.Title)
		titleScore := CalculateSimilarity(originalCleanTitle, cleanResultTitle)
		artistScore := 0.0
		for _, originalArtist := range originalArtists {
			for _, resultArtist := range cleanCollaboratingArtists(result.Artist) {
				if similarity := CalculateSimilarity(originalArtist, resultArtist); similarity > artistScore {
					artistScore = similarity
				}
			}
		}
		log.Printf("Apple Music Match -> Title/Artist %.2f%%/%.2f%% | Duration diff: %d ms (Allowed: %d ms)", titleScore, artistScore, durationDifference, appleMusicDurationToleranceMs)
		if titleScore >= 90.0 && artistScore >= 90.0 {
			log.Printf("SUCCESS: 100%% text match! Bypassing duration check. URL: %s", result.URL)

			if result.URL != "" {
				return result
			}

			log.Printf("ERROR: JSON returned empty trackViewUrl for this track.")
		}
		if originalDurationMs != 0 && result.DurationMs != 0 && durationDifference > appleMusicDurationToleranceMs {
			log.Printf("Apple Music Match -> duration rejected: %d ms (Allowed: %d ms)", durationDifference, appleMusicDurationToleranceMs)
			continue
		}
		if titleScore > 85.0 && artistScore > 80.0 && result.URL != "" {
			return result
		}
	}
	return nil
}

func findBestAppleMusicMatchForSource(originalDurationMs int, originalCleanTitle, originalCleanArtist, sourcePlatform string, results []TrackResult) *TrackResult {
	if strings.EqualFold(sourcePlatform, "YouTube Music") {
		for index := range results {
			result := &results[index]
			if CalculateSimilarity(originalCleanTitle, CleanTrackTitle(result.Title)) >= 75.0 && result.URL != "" {
				return result
			}
			durationDifference := originalDurationMs - result.DurationMs
			if durationDifference < 0 {
				durationDifference = -durationDifference
			}
			if originalDurationMs != 0 && result.DurationMs != 0 && durationDifference > appleMusicDurationToleranceMs {
				continue
			}
		}
		return nil
	}
	return findBestAppleMusicMatch(originalDurationMs, originalCleanTitle, originalCleanArtist, results)
}

func appleMusicTrackID(trackURL string) (string, error) {
	parsedURL, err := url.Parse(trackURL)
	if err != nil {
		return "", fmt.Errorf("invalid Apple Music URL: %w", err)
	}
	trackID := parsedURL.Query().Get("i")
	if trackID == "" {
		return "", fmt.Errorf("Apple Music track ID not found in URL")
	}
	if _, err := strconv.ParseInt(trackID, 10, 64); err != nil {
		return "", fmt.Errorf("invalid Apple Music track ID: %w", err)
	}
	return trackID, nil
}
