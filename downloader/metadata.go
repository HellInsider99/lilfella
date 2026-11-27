package downloader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// readMetadataText returns post text from the metadata JSON files that
// gallery-dl (--write-metadata, "<file>.json") and yt-dlp (--write-info-json,
// ".info.json") save next to downloads. Files are read in name order, and
// in each file the keys are tried in order; the first non-empty string value
// wins. It returns "" if nothing is found.
func readMetadataText(dir string, keys ...string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var metadata map[string]any
		if json.Unmarshal(data, &metadata) != nil {
			continue
		}
		for _, key := range keys {
			if value, ok := metadata[key].(string); ok {
				if text := strings.TrimSpace(value); text != "" {
					return text
				}
			}
		}
	}
	return ""
}
