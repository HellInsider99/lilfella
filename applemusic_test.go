package main

import "testing"

func TestAppleMusicTrackID(t *testing.T) {
	got, err := appleMusicTrackID("https://music.apple.com/us/album/example/123456789?i=987654321")
	if err != nil {
		t.Fatalf("appleMusicTrackID() error = %v", err)
	}
	if got != "987654321" {
		t.Fatalf("appleMusicTrackID() = %q, want %q", got, "987654321")
	}
}

func TestAppleMusicTrackIDRequiresQueryParameter(t *testing.T) {
	if _, err := appleMusicTrackID("https://music.apple.com/us/album/example/123456789"); err == nil {
		t.Fatal("appleMusicTrackID() error = nil, want missing i parameter error")
	}
}

func TestAppleMusicSearchUsesTrackValidation(t *testing.T) {
	platform := NewAppleMusicPlatform()
	results := []TrackResult{
		{Title: "Song", Artist: "Artist", DurationMs: 200000, URL: "https://music.apple.com/song"},
		{Title: "Wrong Song", Artist: "Artist", DurationMs: 200000, URL: "https://music.apple.com/wrong"},
	}
	match := FindBestMatch(200000, "song", "artist", results)
	if match == nil || match.URL != "https://music.apple.com/song" {
		t.Fatalf("FindBestMatch() = %#v, want Apple Music candidate", match)
	}
	if platform.Name() != "Apple Music" {
		t.Fatalf("Apple Music platform name = %q", platform.Name())
	}
}

func TestFindBestAppleMusicMatchAllowsFifteenSecondDifference(t *testing.T) {
	results := []TrackResult{{
		Title:      "Song",
		Artist:     "Artist",
		DurationMs: 215000,
		URL:        "https://music.apple.com/song",
	}}

	match := findBestAppleMusicMatch(200000, "song", "artist", results)
	if match == nil {
		t.Fatal("findBestAppleMusicMatch() = nil, want match at 15-second boundary")
	}
}

func TestFindBestAppleMusicMatchIgnoresMissingDuration(t *testing.T) {
	results := []TrackResult{{
		Title:      "Song",
		Artist:     "Artist",
		DurationMs: 0,
		URL:        "https://music.apple.com/song",
	}}

	match := findBestAppleMusicMatch(200000, "song", "artist", results)
	if match == nil {
		t.Fatal("findBestAppleMusicMatch() = nil, want match when result duration is missing")
	}
}

func TestFindBestAppleMusicMatchRejectsOverFifteenSeconds(t *testing.T) {
	results := []TrackResult{{
		Title:      "Song",
		Artist:     "Artist",
		DurationMs: 215001,
	}}

	if match := findBestAppleMusicMatch(200000, "song", "artist", results); match != nil {
		t.Fatalf("findBestAppleMusicMatch() = %#v, want nil beyond 15-second tolerance", match)
	}
}

func TestFindBestAppleMusicMatchForcesStrongTextMatchPastDuration(t *testing.T) {
	results := []TrackResult{{
		Title:      "Song",
		Artist:     "Artist",
		DurationMs: 300000,
		URL:        "https://music.apple.com/song",
	}}

	match := findBestAppleMusicMatch(200000, "song", "artist", results)
	if match == nil || match.URL != "https://music.apple.com/song" {
		t.Fatalf("findBestAppleMusicMatch() = %#v, want forced strong-text match", match)
	}
}

func TestFindBestAppleMusicMatchRejectsStrongTextWithoutURL(t *testing.T) {
	results := []TrackResult{{Title: "Song", Artist: "Artist", DurationMs: 200000}}
	if match := findBestAppleMusicMatch(200000, "song", "artist", results); match != nil {
		t.Fatalf("findBestAppleMusicMatch() = %#v, want nil without trackViewUrl", match)
	}
}
