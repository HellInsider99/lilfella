package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type SpotifyPlatform struct {
	client       *http.Client
	clientID     string
	clientSecret string
	cache        SpotifyTokenCache
	urlPattern   *regexp.Regexp
}

type SpotifyTokenCache struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

type spotifyTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type spotifyOEmbedResponse struct {
	Title      string `json:"title"`
	AuthorName string `json:"author_name"`
}

type spotifySearchResponse struct {
	Tracks struct {
		Items []struct {
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			DurationMs   int `json:"duration_ms"`
			ExternalURLs struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
		} `json:"items"`
	} `json:"tracks"`
}

type spotifyTrackResponse struct {
	Name    string `json:"name"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	DurationMs int `json:"duration_ms"`
}

func NewSpotifyPlatform(clientID, clientSecret string) *SpotifyPlatform {
	return &SpotifyPlatform{
		client:       http.DefaultClient,
		clientID:     clientID,
		clientSecret: clientSecret,
		urlPattern:   regexp.MustCompile(`https?://(?:open\.)?spotify\.com/[^\s]+|https?://spotify\.link/[^\s]+`),
	}
}

func (platform *SpotifyPlatform) Name() string {
	return "Spotify"
}

func (platform *SpotifyPlatform) ExtractURL(text string) (string, bool) {
	match := platform.urlPattern.FindString(text)
	return strings.TrimRight(match, ".,!?)]}"), match != ""
}

func (platform *SpotifyPlatform) TrackFromURL(trackURL string) (Track, error) {
	trackIDMatch := regexp.MustCompile(`/track/([A-Za-z0-9]+)`).FindStringSubmatch(trackURL)
	if len(trackIDMatch) < 2 {
		return Track{}, fmt.Errorf("invalid Spotify track URL")
	}
	if platform.clientID == "" || platform.clientSecret == "" {
		return Track{}, fmt.Errorf("Spotify credentials are not set")
	}

	token, err := platform.cache.Get(platform.client, platform.clientID, platform.clientSecret)
	if err != nil {
		return Track{}, err
	}
	request, err := http.NewRequest(http.MethodGet, "https://api.spotify.com/v1/tracks/"+trackIDMatch[1], nil)
	if err != nil {
		return Track{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := platform.client.Do(request)
	if err != nil {
		return Track{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Track{}, fmt.Errorf("Spotify track API returned %s", response.Status)
	}

	var track spotifyTrackResponse
	if err := json.NewDecoder(response.Body).Decode(&track); err != nil {
		return Track{}, err
	}
	artist := ""
	if len(track.Artists) > 0 {
		artist = track.Artists[0].Name
	}
	return Track{Title: track.Name, Artist: artist, DurationMs: track.DurationMs}, nil
}

func (platform *SpotifyPlatform) SearchTrack(track Track) (string, error) {
	if platform.clientID == "" || platform.clientSecret == "" {
		return "", fmt.Errorf("Spotify credentials are not set")
	}

	token, err := platform.cache.Get(platform.client, platform.clientID, platform.clientSecret)
	if err != nil {
		return "", err
	}

	searchURL := "https://api.spotify.com/v1/search?" + url.Values{
		"type":  {"track"},
		"limit": {"10"},
		"q":     {strings.TrimSpace(track.Artist + " " + track.Title)},
	}.Encode()
	request, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := platform.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Spotify search API returned %s", response.Status)
	}

	var searchResponse spotifySearchResponse
	if err := json.NewDecoder(response.Body).Decode(&searchResponse); err != nil {
		return "", err
	}
	results := make([]TrackResult, 0, len(searchResponse.Tracks.Items))
	for _, item := range searchResponse.Tracks.Items {
		if item.ExternalURLs.Spotify == "" || len(item.Artists) == 0 {
			continue
		}
		results = append(results, TrackResult{
			Title:      item.Name,
			Artist:     item.Artists[0].Name,
			DurationMs: item.DurationMs,
			URL:        item.ExternalURLs.Spotify,
		})
	}
	match := FindBestMatchForSource(track.DurationMs, CleanTrackTitle(track.Title), CleanArtistName(track.Artist), track.SourcePlatform, results)
	if match == nil {
		return "", fmt.Errorf("Spotify track not found")
	}
	return match.URL, nil
}

func (cache *SpotifyTokenCache) Get(client *http.Client, clientID, clientSecret string) (string, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if cache.token != "" && time.Now().Before(cache.expiresAt) {
		return cache.token, nil
	}

	request, err := http.NewRequest(http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+clientSecret)))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Spotify token API returned %s", response.Status)
	}

	var tokenResponse spotifyTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}
	if tokenResponse.AccessToken == "" {
		return "", fmt.Errorf("Spotify token API returned an empty access token")
	}

	cache.token = tokenResponse.AccessToken
	cache.expiresAt = time.Now().Add(59 * time.Minute)
	return cache.token, nil
}
