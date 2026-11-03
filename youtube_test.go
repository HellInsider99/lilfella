package main

import "testing"

func TestParseYouTubeTrackName(t *testing.T) {
	tests := []struct {
		name       string
		rawTitle   string
		fallback   string
		wantTitle  string
		wantArtist string
	}{
		{name: "middle dot", rawTitle: "Track Name · Artist Name", wantTitle: "Track Name", wantArtist: "Artist Name"},
		{name: "hyphen", rawTitle: "Track Name - Artist Name", wantTitle: "Track Name", wantArtist: "Artist Name"},
		{name: "oembed fallback", rawTitle: "Track Name", fallback: "Artist Name", wantTitle: "Track Name", wantArtist: "Artist Name"},
		{name: "fallback artist overrides release suffix", rawTitle: "Behemoth (Remix) - Release", fallback: "Gost & Perturbator", wantTitle: "Behemoth (Remix)", wantArtist: "Gost & Perturbator"},
		{name: "artist on left", rawTitle: "Artist Name - Track Name", fallback: "Artist Name", wantTitle: "Track Name", wantArtist: "Artist Name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotTitle, gotArtist := parseYouTubeTrackName(test.rawTitle, test.fallback)
			if gotTitle != test.wantTitle || gotArtist != test.wantArtist {
				t.Fatalf("parseYouTubeTrackName() = %q, %q; want %q, %q", gotTitle, gotArtist, test.wantTitle, test.wantArtist)
			}
		})
	}
}

func TestYouTubeTrackMetadataIsCleaned(t *testing.T) {
	title, artist := parseYouTubeTrackName("Track Name (Official Video) · Artist Name - Topic", "")
	if got := CleanTrackTitle(title); got != "track name" {
		t.Fatalf("CleanTrackTitle() = %q, want %q", got, "track name")
	}
	if got := CleanArtistName(artist); got != "artist name" {
		t.Fatalf("CleanArtistName() = %q, want %q", got, "artist name")
	}
}

func TestExtractYouTubeVideoID(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "watch", url: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "short", url: "https://youtu.be/dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
		{name: "music", url: "https://music.youtube.com/watch?v=dQw4w9WgXcQ", want: "dQw4w9WgXcQ"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := extractYouTubeVideoID(test.url)
			if err != nil || got != test.want {
				t.Fatalf("extractYouTubeVideoID() = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestYouTubeArtistLooksSuspicious(t *testing.T) {
	for _, artist := range []string{"Release", "Provided to YouTube", "Official Audio"} {
		if !youtubeArtistLooksSuspicious(artist) {
			t.Errorf("youtubeArtistLooksSuspicious(%q) = false, want true", artist)
		}
	}
	if youtubeArtistLooksSuspicious("Gost & Perturbator") {
		t.Error("youtubeArtistLooksSuspicious(valid artist) = true, want false")
	}
	if youtubeArtistLooksSuspicious("Gost - Topic") {
		t.Error("youtubeArtistLooksSuspicious(Topic channel) = true, want false")
	}
}

func TestCleanYouTubeArtistName(t *testing.T) {
	tests := map[string]string{
		"Gost - Topic":               "Gost",
		"Release - Gost":             "Gost",
		"Release - Topic":            "Release",
		"Gost & Perturbator - Topic": "Gost & Perturbator",
	}
	for input, want := range tests {
		if got := cleanYouTubeArtistName(input); got != want {
			t.Errorf("cleanYouTubeArtistName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseYouTubeDuration(t *testing.T) {
	got, err := parseYouTubeDuration("PT1H2M3S")
	if err != nil {
		t.Fatalf("parseYouTubeDuration() error = %v", err)
	}
	if got != 3723000 {
		t.Fatalf("parseYouTubeDuration() = %d, want 3723000", got)
	}
}

func TestFindBestYouTubeMatch(t *testing.T) {
	results := []TrackResult{
		{Title: "Artist Name - Track Name (Official Video)", DurationMs: 202000, URL: "https://example.com/video"},
		{Title: "Track Name", DurationMs: 206001, URL: "https://example.com/too-long"},
	}

	match := FindBestYouTubeMatch(200000, "track name", results)
	if match == nil || match.URL != "https://example.com/video" {
		t.Fatalf("FindBestYouTubeMatch() = %#v, want first result", match)
	}

	if match := FindBestYouTubeMatch(200000, "track name", []TrackResult{{Title: "Artist Name - Different Track", DurationMs: 200000}}); match != nil {
		t.Fatalf("FindBestYouTubeMatch() = %#v, want nil for title mismatch", match)
	}
}
