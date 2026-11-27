package downloader

import "testing"

func TestValidateTikTokURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "standard video", url: "https://www.tiktok.com/@user/video/1234567890"},
		{name: "mobile video with query", url: "https://m.tiktok.com/@user/video/1234567890?lang=en"},
		{name: "photo post", url: "https://www.tiktok.com/@user/photo/1234567890"},
		{name: "short share path", url: "https://www.tiktok.com/t/ZM123456/"},
		{name: "short link", url: "https://vm.tiktok.com/ZM123456/"},
		{name: "alternate short link", url: "https://vt.tiktok.com/ZM123456/"},
		{name: "non-video path", url: "https://www.tiktok.com/@user", wantErr: true},
		{name: "empty short share path", url: "https://www.tiktok.com/t/", wantErr: true},
		{name: "untrusted host", url: "https://tiktok.com.example.org/@user/video/123", wantErr: true},
		{name: "unsupported scheme", url: "ftp://www.tiktok.com/@user/video/123", wantErr: true},
		{name: "empty short link", url: "https://vm.tiktok.com/", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTikTokURL(test.url)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateTikTokURL(%q) error = %v, wantErr %t", test.url, err, test.wantErr)
			}
		})
	}
}

func TestTikTokVideoRejectsInvalidURL(t *testing.T) {
	path, cleanup, err := TikTokVideo("https://example.com/video")
	if err == nil {
		cleanup()
		t.Fatal("TikTokVideo() error = nil, want unsupported host error")
	}
	if path != "" {
		t.Errorf("TikTokVideo() path = %q, want empty", path)
	}
}

func TestIsTikTokPhotoURL(t *testing.T) {
	tests := map[string]bool{
		"https://www.tiktok.com/@user/photo/1234567890?lang=en": true,
		"https://www.tiktok.com/@user/video/1234567890":         false,
		"https://vm.tiktok.com/ZM123456/":                       false,
	}
	for rawURL, want := range tests {
		if got := IsTikTokPhotoURL(rawURL); got != want {
			t.Errorf("IsTikTokPhotoURL(%q) = %t, want %t", rawURL, got, want)
		}
	}
}
