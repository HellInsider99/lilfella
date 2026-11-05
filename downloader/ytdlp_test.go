package downloader

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestYtDlpCookiesArgs(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		t.Setenv("YTDLP_COOKIES_FILE", "")

		args, err := ytDlpCookiesArgs()
		if err != nil {
			t.Fatalf("ytDlpCookiesArgs() error = %v", err)
		}
		if len(args) != 0 {
			t.Fatalf("ytDlpCookiesArgs() = %v, want no arguments", args)
		}
	})

	t.Run("valid file", func(t *testing.T) {
		cookiesPath := filepath.Join(t.TempDir(), "cookies.txt")
		if err := os.WriteFile(cookiesPath, []byte("# Netscape HTTP Cookie File\n"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("YTDLP_COOKIES_FILE", cookiesPath)

		args, err := ytDlpCookiesArgs()
		if err != nil {
			t.Fatalf("ytDlpCookiesArgs() error = %v", err)
		}
		want := []string{"--cookies", cookiesPath}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("ytDlpCookiesArgs() = %v, want %v", args, want)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		t.Setenv("YTDLP_COOKIES_FILE", filepath.Join(t.TempDir(), "missing.txt"))

		if _, err := ytDlpCookiesArgs(); err == nil {
			t.Fatal("ytDlpCookiesArgs() error = nil, want missing-file error")
		}
	})

	t.Run("directory", func(t *testing.T) {
		t.Setenv("YTDLP_COOKIES_FILE", t.TempDir())

		if _, err := ytDlpCookiesArgs(); err == nil {
			t.Fatal("ytDlpCookiesArgs() error = nil, want regular-file error")
		}
	})
}
