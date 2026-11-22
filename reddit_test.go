package main

import "testing"

func TestFindDownloadableMediaReddit(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantURL string
	}{
		{name: "post", text: "https://www.reddit.com/r/golang/comments/abc123/example/", wantURL: "https://www.reddit.com/r/golang/comments/abc123/example/"},
		{name: "old post", text: "https://old.reddit.com/r/golang/comments/abc123/example/", wantURL: "https://old.reddit.com/r/golang/comments/abc123/example/"},
		{name: "gallery", text: "https://reddit.com/r/golang/gallery/abc123", wantURL: "https://reddit.com/r/golang/gallery/abc123"},
		{name: "short link", text: "https://redd.it/abc123", wantURL: "https://redd.it/abc123"},
		{name: "punctuation", text: "дивись тут: https://redd.it/abc123.", wantURL: "https://redd.it/abc123"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotURL, gotType, ok := findDownloadableMedia(test.text)
			if gotURL != test.wantURL || gotType != "reddit" || !ok {
				t.Fatalf("findDownloadableMedia(%q) = %q, %q, %t; want %q, reddit, true",
					test.text, gotURL, gotType, ok, test.wantURL)
			}
		})
	}
}
