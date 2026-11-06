package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstagramShortcode(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "reel", url: "https://www.instagram.com/reel/ABC_123-def/", want: "ABC_123-def"},
		{name: "reels path", url: "https://www.instagram.com/reels/DesoBEUsO2_/", want: "DesoBEUsO2_"},
		{name: "mobile reel", url: "https://m.instagram.com/reel/ABC_123-def/", want: "ABC_123-def"},
		{name: "post", url: "https://instagram.com/p/ABC123/?img_index=1", want: "ABC123"},
		{name: "not an Instagram post", url: "https://instagram.com/accounts/login/", want: ""},
		{name: "empty", url: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := instagramShortcode(test.url); got != test.want {
				t.Fatalf("instagramShortcode(%q) = %q, want %q", test.url, got, test.want)
			}
		})
	}
}

func TestInstaloaderPython(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "configured interpreter", env: `C:\bot\venv\Scripts\python.exe`, want: `C:\bot\venv\Scripts\python.exe`},
		{name: "default interpreter", env: "", want: "python3"},
		{name: "whitespace uses default", env: "  ", want: "python3"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("INSTALOADER_PYTHON", test.env)
			if got := instaloaderPython(); got != test.want {
				t.Fatalf("instaloaderPython() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInstagramReelRejectsURLWithoutShortcode(t *testing.T) {
	path, cleanup, err := InstagramReel("https://www.instagram.com/accounts/login/")
	if err == nil {
		cleanup()
		t.Fatal("InstagramReel() error = nil, want missing-shortcode error")
	}
	if path != "" {
		t.Errorf("InstagramReel() path = %q, want empty", path)
	}
}

func TestFindFileWithExtension(t *testing.T) {
	root := t.TempDir()
	videoDir := filepath.Join(root, "profile")
	if err := os.Mkdir(videoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	videoPath := filepath.Join(videoDir, "post.mp4")
	if err := os.WriteFile(filepath.Join(videoDir, "post.jpg"), []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(videoPath, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := findFileWithExtension(root, ".mp4")
	if err != nil {
		t.Fatalf("findFileWithExtension() error = %v", err)
	}
	if got != videoPath {
		t.Errorf("findFileWithExtension() = %q, want %q", got, videoPath)
	}
}

func TestFindFileWithExtensionReturnsErrorWhenMissing(t *testing.T) {
	if _, err := findFileWithExtension(t.TempDir(), ".mp4"); err == nil {
		t.Fatal("findFileWithExtension() error = nil, want missing-file error")
	}
}
