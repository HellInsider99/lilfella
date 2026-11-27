package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMetadataText(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"001.jpg":      "x",
		"000.bad.json": "not json",
		"001.jpg.json": `{"description": "  Hello from the post  ", "desc": "tiktok text", "num": 1}`,
		"002.mp4.json": `{"description": "second"}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if got := readMetadataText(dir, "description"); got != "Hello from the post" {
		t.Errorf("readMetadataText(description) = %q, want %q", got, "Hello from the post")
	}
	if got := readMetadataText(dir, "desc", "description"); got != "tiktok text" {
		t.Errorf("readMetadataText(desc, description) = %q, want %q", got, "tiktok text")
	}
	if got := readMetadataText(dir, "title"); got != "" {
		t.Errorf("readMetadataText(missing key) = %q, want empty", got)
	}
	if got := readMetadataText(t.TempDir(), "description"); got != "" {
		t.Errorf("readMetadataText(empty dir) = %q, want empty", got)
	}
}
