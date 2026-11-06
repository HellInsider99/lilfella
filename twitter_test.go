package main

import "testing"

func TestFindDownloadableMediaTwitter(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantURL string
	}{
		{
			name:    "X URL",
			text:    "https://x.com/user/status/123456",
			wantURL: "https://x.com/user/status/123456",
		},
		{
			name:    "Twitter URL",
			text:    "https://twitter.com/user/status/123456",
			wantURL: "https://twitter.com/user/status/123456",
		},
		{
			name:    "vxtwitter URL",
			text:    "https://vxtwitter.com/user/status/123456",
			wantURL: "https://vxtwitter.com/user/status/123456",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotURL, gotType, ok := findDownloadableMedia(test.text)
			if gotURL != test.wantURL || gotType != "twitter" || !ok {
				t.Fatalf("findDownloadableMedia(%q) = %q, %q, %t; want %q, twitter, true",
					test.text, gotURL, gotType, ok, test.wantURL)
			}
		})
	}
}
