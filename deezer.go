package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type DeezerPlatform struct {
	client     *http.Client
	urlPattern *regexp.Regexp
}

type deezerTrackResponse struct {
	Duration int    `json:"duration"`
	Title    string `json:"title"`
	Artist   struct {
		Name string `json:"name"`
	} `json:"artist"`
}

type deezerSearchResponse struct {
	Data []struct {
		Link     string `json:"link"`
		Title    string `json:"title"`
		Duration int    `json:"duration"`
		Artist   struct {
			Name string `json:"name"`
		} `json:"artist"`
	} `json:"data"`
}

func NewDeezerPlatform() *DeezerPlatform {
	return &DeezerPlatform{
		client:     http.DefaultClient,
		urlPattern: regexp.MustCompile(`https?://(?:(?:www|link)\.)?deezer\.(?:com|page\.link)[^\s]+`),
	}
}

func (platform *DeezerPlatform) Name() string {
	return "Deezer"
}

func (platform *DeezerPlatform) ExtractURL(text string) (string, bool) {
	match := platform.urlPattern.FindString(text)
	return strings.TrimRight(match, ".,!?)]}"), match != ""
}

func (platform *DeezerPlatform) TrackFromURL(trackURL string) (Track, error) {
	request, err := http.NewRequest(http.MethodGet, trackURL, nil)
	if err != nil {
		return Track{}, err
	}
	response, err := platform.client.Do(request)
	if err != nil {
		return Track{}, err
	}
	response.Body.Close()

	match := regexp.MustCompile(`/track/(\d+)`).FindStringSubmatch(response.Request.URL.String())
	if len(match) < 2 {
		return Track{}, fmt.Errorf("invalid Deezer track URL")
	}

	apiURL := "https://api.deezer.com/track/" + match[1]
	response, err = platform.client.Get(apiURL)
	if err != nil {
		return Track{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Track{}, fmt.Errorf("Deezer API returned %s", response.Status)
	}

	var track deezerTrackResponse
	if err := json.NewDecoder(response.Body).Decode(&track); err != nil {
		return Track{}, err
	}
	return Track{Title: track.Title, Artist: track.Artist.Name, DurationMs: track.Duration * 1000}, nil
}

func (platform *DeezerPlatform) SearchTrack(track Track) (string, error) {
	artist := strings.TrimSpace(track.Artist)
	title := strings.TrimSpace(track.Title)
	searchTerm := strings.TrimSpace(artist + " " + title)
	if artist == "" {
		searchTerm = deezerTitleSearchTerm(title)
	}
	results, err := platform.searchResults(searchTerm)
	if err != nil {
		return "", err
	}
	if len(results) == 0 && artist != "" {
		results, err = platform.searchResults(deezerTitleSearchTerm(title))
		if err != nil {
			return "", err
		}
	}
	match := FindBestMatchForSource(track.DurationMs, CleanTrackTitle(track.Title), CleanArtistName(track.Artist), track.SourcePlatform, results)
	if match == nil {
		return "", fmt.Errorf("Deezer track not found")
	}
	return match.URL, nil
}

func deezerTitleSearchTerm(title string) string {
	return `track:"` + strings.ReplaceAll(title, `"`, `\"`) + `"`
}

func (platform *DeezerPlatform) searchResults(term string) ([]TrackResult, error) {
	query := url.Values{"q": {term}, "limit": {"25"}}.Encode()
	response, err := platform.client.Get("https://api.deezer.com/search?" + query)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Deezer search API returned %s", response.Status)
	}
	var searchResponse deezerSearchResponse
	if err := json.NewDecoder(response.Body).Decode(&searchResponse); err != nil {
		return nil, err
	}
	results := make([]TrackResult, 0, len(searchResponse.Data))
	for _, item := range searchResponse.Data {
		if item.Link == "" {
			continue
		}
		results = append(results, TrackResult{Title: item.Title, Artist: item.Artist.Name, DurationMs: item.Duration * 1000, URL: item.Link})
	}
	return results, nil
}
