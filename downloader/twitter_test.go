package downloader

import (
	"encoding/json"
	"testing"
)

func TestParseTwitterMediaPrefersExtendedTypes(t *testing.T) {
	payload := twitterResponse{
		MediaURLs: json.RawMessage(`["https://video.twimg.com/clip.mp4"]`),
		MediaExtended: []TwitterMediaItem{
			{URL: "https://pbs.twimg.com/media/photo.jpg", Type: "image"},
			{URL: "https://video.twimg.com/clip.mp4", Type: "video"},
		},
	}

	got := parseTwitterMedia(payload)
	if len(got) != 2 {
		t.Fatalf("parseTwitterMedia() returned %d items, want 2", len(got))
	}
	if got[0].Type != "image" || got[1].Type != "video" {
		t.Errorf("parseTwitterMedia() types = %q, %q; want image, video", got[0].Type, got[1].Type)
	}
}

func TestParseTwitterMediaInfersLegacyURLs(t *testing.T) {
	payload := twitterResponse{
		MediaURLs: json.RawMessage(`[
			"https://pbs.twimg.com/media/photo.jpg?format=jpg&name=large",
			"https://video.twimg.com/ext_tw_video/123/pu/vid/avc1/720x1280/video.mp4"
		]`),
	}

	got := parseTwitterMedia(payload)
	if len(got) != 2 {
		t.Fatalf("parseTwitterMedia() returned %d items, want 2", len(got))
	}
	if got[0].Type != "image" || got[1].Type != "video" {
		t.Errorf("parseTwitterMedia() types = %q, %q; want image, video", got[0].Type, got[1].Type)
	}
}

func TestParseTwitterMediaSingleImage(t *testing.T) {
	payload := twitterResponse{MediaURL: "https://pbs.twimg.com/media/photo.jpg"}

	got := parseTwitterMedia(payload)
	if len(got) != 1 || got[0].Type != "image" {
		t.Fatalf("parseTwitterMedia() = %#v, want one image", got)
	}
}
