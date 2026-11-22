package downloader

import (
	"encoding/json"
	"testing"
)

func TestValidateRedditURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "post", url: "https://www.reddit.com/r/golang/comments/abc123/example/"},
		{name: "gallery", url: "https://www.reddit.com/r/golang/gallery/abc123"},
		{name: "short", url: "https://redd.it/abc123"},
		{name: "old host", url: "https://old.reddit.com/r/golang/comments/abc123/example/"},
		{name: "untrusted host", url: "https://reddit.com.example.org/r/golang/comments/abc123/", wantErr: true},
		{name: "unsupported scheme", url: "ftp://reddit.com/r/golang/comments/abc123/", wantErr: true},
		{name: "empty short", url: "https://redd.it/", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRedditURL(test.url)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateRedditURL(%q) error = %v, wantErr %t", test.url, err, test.wantErr)
			}
		})
	}
}

func TestParseRedditMediaGallery(t *testing.T) {
	var payload []redditPostResponse
	if err := json.Unmarshal([]byte(`[
		{"data":{"children":[{"data":{"gallery_data":{"items":[{"media_id":"one"},{"media_id":"two"}]},"media_metadata":{
			"one":{"status":"valid","s":{"u":"https://preview.redd.it/one.jpg?width=1080&amp;format=pjpg"}},
			"two":{"status":"valid","s":{"u":"https://preview.redd.it/two.jpg?width=1080&amp;format=pjpg"}}
		}}}]}}
	]`), &payload); err != nil {
		t.Fatal(err)
	}

	items := parseRedditMedia(payload)
	if len(items) != 2 {
		t.Fatalf("parseRedditMedia() returned %d items, want 2", len(items))
	}
	if items[0].Type != "image" || items[0].URL != "https://preview.redd.it/one.jpg?width=1080&format=pjpg" {
		t.Errorf("first item = %#v", items[0])
	}
}

func TestParseRedditMediaVideo(t *testing.T) {
	var payload []redditPostResponse
	if err := json.Unmarshal([]byte(`[{"data":{"children":[{"data":{"is_video":true}}]}}]`), &payload); err != nil {
		t.Fatal(err)
	}
	items := parseRedditMedia(payload)
	if len(items) != 1 || items[0].Type != "video" {
		t.Fatalf("parseRedditMedia() = %#v, want video item", items)
	}
}
