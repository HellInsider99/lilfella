package main

import "testing"

func TestCleanTrackTitle(t *testing.T) {
	got := CleanTrackTitle("  Artist Song (Official Video) [Lyrics] - Music Video ")
	want := "artist song -"
	if got != want {
		t.Fatalf("CleanTrackTitle() = %q, want %q", got, want)
	}
}

func TestCleanTrackTitlePreservesValidParentheses(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "The Beast II (1996)", want: "the beast ii (1996)"},
		{input: "Song (feat. Another Artist)", want: "song"},
		{input: "Song [Deluxe Edition]", want: "song [deluxe edition]"},
		{input: "Space - with Another Artist", want: "space"},
		{input: "Track [prod. Someone]", want: "track"},
	}

	for _, test := range tests {
		if got := CleanTrackTitle(test.input); got != test.want {
			t.Errorf("CleanTrackTitle(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestCleanArtistName(t *testing.T) {
	got := CleanArtistName("  Example Artist - Topic (Official) [VEVO] ")
	want := "example artist"
	if got != want {
		t.Fatalf("CleanArtistName() = %q, want %q", got, want)
	}
}

func TestSanitizeSearchArtist(t *testing.T) {
	for _, junk := range []string{"Release", "TOPIC", "Release - Topic", "Various Artists"} {
		if got := sanitizeSearchArtist(junk); got != "" {
			t.Errorf("sanitizeSearchArtist(%q) = %q, want empty", junk, got)
		}
	}
	if got := sanitizeSearchArtist("Gost & Perturbator"); got != "Gost & Perturbator" {
		t.Errorf("sanitizeSearchArtist(valid artist) = %q", got)
	}
}

func TestSplitCollaboratingArtists(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{input: "gost & perturbator", want: []string{"gost", "perturbator"}},
		{input: "gost, perturbator", want: []string{"gost", "perturbator"}},
		{input: "gost feat. perturbator", want: []string{"gost", "perturbator"}},
		{input: "gost ft perturbator", want: []string{"gost", "perturbator"}},
		{input: "gost and perturbator", want: []string{"gost", "perturbator"}},
		{input: "gost x perturbator", want: []string{"gost", "perturbator"}},
		{input: "gost vs perturbator", want: []string{"gost", "perturbator"}},
		{input: "gost vs. perturbator", want: []string{"gost", "perturbator"}},
	}

	for _, test := range tests {
		got := splitCollaboratingArtists(test.input)
		if len(got) != len(test.want) {
			t.Fatalf("splitCollaboratingArtists(%q) = %#v, want %#v", test.input, got, test.want)
		}
		for index := range got {
			if got[index] != test.want[index] {
				t.Errorf("splitCollaboratingArtists(%q)[%d] = %q, want %q", test.input, index, got[index], test.want[index])
			}
		}
	}
}

func TestCleanCollaboratingArtistsBeforeValidation(t *testing.T) {
	got := cleanCollaboratingArtists("Gost & Perturbator - Topic")
	want := []string{"gost", "perturbator"}
	if len(got) != len(want) {
		t.Fatalf("cleanCollaboratingArtists() = %#v, want %#v", got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Errorf("cleanCollaboratingArtists()[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestCleanCollaboratingArtistsNormalizesAPIDelimiters(t *testing.T) {
	got := cleanCollaboratingArtists("Gost, Perturbator and Author x Producer vs Guest")
	want := []string{"gost", "perturbator", "author", "producer", "guest"}
	if len(got) != len(want) {
		t.Fatalf("cleanCollaboratingArtists() = %#v, want %#v", got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Errorf("cleanCollaboratingArtists()[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}

func TestCalculateSimilarity(t *testing.T) {
	if got := CalculateSimilarity("", ""); got != 100.0 {
		t.Fatalf("empty strings similarity = %v, want 100", got)
	}
	if got := CalculateSimilarity("abc", "abd"); got < 66.66 || got > 66.67 {
		t.Fatalf("CalculateSimilarity() = %v, want approximately 66.67", got)
	}
	if got := CalculateSimilarity("Привіт", "Привіт"); got != 100.0 {
		t.Fatalf("Unicode similarity = %v, want 100", got)
	}
}

func TestFindBestMatch(t *testing.T) {
	results := []TrackResult{
		{Title: "Song (Official Video)", Artist: "Example Artist - Topic", DurationMs: 203000, URL: "https://example.com/song"},
		{Title: "Song", Artist: "Example Artist", DurationMs: 206001, URL: "https://example.com/too-long"},
	}

	match := FindBestMatch(200000, "song", "example artist", results)
	if match == nil || match.URL != "https://example.com/song" {
		t.Fatalf("FindBestMatch() = %#v, want first result", match)
	}

	if match := FindBestMatch(200000, "song", "example artist", []TrackResult{{Title: "Song", Artist: "Wrong Artist", DurationMs: 210001}}); match != nil {
		t.Fatalf("FindBestMatch() = %#v, want nil for duration difference over 10 seconds", match)
	}

	if match := FindBestMatch(200000, "song", "example artist", []TrackResult{{Title: "Song", Artist: "Example Artist", DurationMs: 210000}}); match == nil {
		t.Fatal("FindBestMatch() = nil, want match at the 10-second tolerance boundary")
	}

	if match := FindBestMatch(200000, "song", "example artist", []TrackResult{{Title: "Song", Artist: "Wrong Artist", DurationMs: 200000}}); match != nil {
		t.Fatalf("FindBestMatch() = %#v, want nil for artist mismatch", match)
	}

	if match := FindBestMatch(200000, "song", "gost & perturbator", []TrackResult{{Title: "Song", Artist: "Gost", DurationMs: 200000, URL: "https://example.com/collab"}}); match == nil || match.URL != "https://example.com/collab" {
		t.Fatalf("FindBestMatch() = %#v, want collaboration match", match)
	}

	if match := FindBestMatch(200000, "song", "gost", []TrackResult{{Title: "Song", Artist: "Gost & Perturbator", DurationMs: 200000, URL: "https://example.com/api-collab"}}); match == nil || match.URL != "https://example.com/api-collab" {
		t.Fatalf("FindBestMatch() = %#v, want API collaboration match", match)
	}

	if match := FindBestMatch(200000, "song remix", "example artist", []TrackResult{{Title: "Song Remix - Extended Club Version", Artist: "Example Artist", DurationMs: 202000, URL: "https://example.com/remix"}}); match == nil || match.URL != "https://example.com/remix" {
		t.Fatalf("FindBestMatch() = %#v, want remix match", match)
	}
}

func TestFindBestMatchForSourceRelaxesYouTubeArtistValidation(t *testing.T) {
	results := []TrackResult{{
		Title:      "Song",
		Artist:     "Different Artist",
		DurationMs: 200000,
		URL:        "https://example.com/song",
	}}

	if match := FindBestMatchForSource(200000, "song", "original artist", "YouTube Music", results); match == nil {
		t.Fatal("FindBestMatchForSource() = nil for YouTube title match, want result")
	}
	if match := FindBestMatchForSource(200000, "song", "original artist", "Spotify", results); match != nil {
		t.Fatalf("FindBestMatchForSource() = %#v for Spotify artist mismatch, want nil", match)
	}
}

func TestDeezerTitleSearchTerm(t *testing.T) {
	if got := deezerTitleSearchTerm("Behemoth (Remix)"); got != `track:"Behemoth (Remix)"` {
		t.Fatalf("deezerTitleSearchTerm() = %q", got)
	}
}
