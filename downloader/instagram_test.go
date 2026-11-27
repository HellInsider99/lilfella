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
		{name: "untrusted host", url: "https://instagram.com.example.org/reel/ABC123/", want: ""},
		{name: "unsupported scheme", url: "ftp://instagram.com/reel/ABC123/", want: ""},
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

func TestCollectInstagramMediaOrdersAndClassifies(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"003.webp", "001.jpg", "002.mp4", "004.part", "notes.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := collectInstagramMedia(dir)
	if err != nil {
		t.Fatalf("collectInstagramMedia() error = %v", err)
	}
	want := []InstagramMediaItem{
		{Path: filepath.Join(dir, "001.jpg"), Type: "image"},
		{Path: filepath.Join(dir, "002.mp4"), Type: "video"},
		{Path: filepath.Join(dir, "003.webp"), Type: "image"},
	}
	if len(got) != len(want) {
		t.Fatalf("collectInstagramMedia() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestCollectInstagramMediaReturnsErrorWhenEmpty(t *testing.T) {
	if _, err := collectInstagramMedia(t.TempDir()); err == nil {
		t.Fatal("collectInstagramMedia() error = nil, want no-media error")
	}
}

func TestInstagramMediaRejectsURLWithoutShortcode(t *testing.T) {
	if _, _, err := InstagramMedia("https://www.instagram.com/accounts/login/"); err == nil {
		t.Fatal("InstagramMedia() error = nil, want missing-shortcode error")
	}
}

func TestReadInstagramDescription(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"001.jpg":      "x",
		"000.bad.json": "not json",
		"001.jpg.json": `{"description": "  Hello from the post  ", "num": 1}`,
		"002.mp4.json": `{"description": "second"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if got := readInstagramDescription(dir); got != "Hello from the post" {
		t.Errorf("readInstagramDescription() = %q, want %q", got, "Hello from the post")
	}
	if got := readInstagramDescription(t.TempDir()); got != "" {
		t.Errorf("readInstagramDescription(empty dir) = %q, want empty", got)
	}
}
