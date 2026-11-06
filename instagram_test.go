package main

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestFindDownloadableMediaInstagram(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantURL   string
		wantType  string
		wantFound bool
	}{
		{
			name:      "reel URL",
			text:      "Check this out https://www.instagram.com/reel/ABC123/?igsh=abc",
			wantURL:   "https://www.instagram.com/reel/ABC123/?igsh=abc",
			wantType:  "instagram",
			wantFound: true,
		},
		{
			name:      "reels URL",
			text:      "https://www.instagram.com/reels/DesoBEUsO2_/",
			wantURL:   "https://www.instagram.com/reels/DesoBEUsO2_/",
			wantType:  "instagram",
			wantFound: true,
		},
		{
			name:      "mobile Instagram reel URL",
			text:      "https://m.instagram.com/reel/ABC123/",
			wantURL:   "https://m.instagram.com/reel/ABC123/",
			wantType:  "instagram",
			wantFound: true,
		},
		{
			name:      "post URL with trailing punctuation",
			text:      "https://www.instagram.com/p/ABC123/.",
			wantURL:   "https://www.instagram.com/p/ABC123/",
			wantType:  "instagram",
			wantFound: true,
		},
		{
			name: "unsupported Instagram URL",
			text: "https://www.instagram.com/stories/user/123/",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotURL, gotType, ok := findDownloadableMedia(test.text)
			if gotURL != test.wantURL || gotType != test.wantType || ok != test.wantFound {
				t.Fatalf("findDownloadableMedia(%q) = %q, %q, %t; want %q, %q, %t",
					test.text, gotURL, gotType, ok, test.wantURL, test.wantType, test.wantFound)
			}
		})
	}
}

func TestFindDownloadableMediaInMessageTextLink(t *testing.T) {
	message := &tgbotapi.Message{
		Text: "ось рілс",
		Entities: []tgbotapi.MessageEntity{
			{Type: "text_link", URL: "https://www.instagram.com/reel/ABC123/"},
		},
	}

	gotURL, gotType, ok := findDownloadableMediaInMessage(message)
	if gotURL != "https://www.instagram.com/reel/ABC123/" || gotType != "instagram" || !ok {
		t.Fatalf("findDownloadableMediaInMessage() = %q, %q, %t; want Instagram reel URL", gotURL, gotType, ok)
	}
}
