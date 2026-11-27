package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectMediaFilesOrdersAndClassifies(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"003.webp", "001.jpg", "002.mp4", "004.part", "notes.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := collectMediaFiles(dir)
	if err != nil {
		t.Fatalf("collectMediaFiles() error = %v", err)
	}
	want := []MediaFile{
		{Path: filepath.Join(dir, "001.jpg"), Type: "image"},
		{Path: filepath.Join(dir, "002.mp4"), Type: "video"},
		{Path: filepath.Join(dir, "003.webp"), Type: "image"},
	}
	if len(got) != len(want) {
		t.Fatalf("collectMediaFiles() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestCollectMediaFilesReturnsErrorWhenEmpty(t *testing.T) {
	if _, err := collectMediaFiles(t.TempDir()); err == nil {
		t.Fatal("collectMediaFiles() error = nil, want no-media error")
	}
}
