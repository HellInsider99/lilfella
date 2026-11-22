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

		args, cleanup, err := ytDlpCookiesArgs()
		cleanup()
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

		args, cleanup, err := ytDlpCookiesArgs()
		if err != nil {
			t.Fatalf("ytDlpCookiesArgs() error = %v", err)
		}
		defer cleanup()
		if len(args) != 2 || args[0] != "--cookies" || args[1] == cookiesPath {
			t.Fatalf("ytDlpCookiesArgs() = %v, want --cookies and a writable copy", args)
		}
		copiedContent, err := os.ReadFile(args[1])
		if err != nil {
			t.Fatalf("read writable cookie copy: %v", err)
		}
		if !reflect.DeepEqual(copiedContent, []byte("# Netscape HTTP Cookie File\n")) {
			t.Fatalf("cookie copy = %q, want original contents", copiedContent)
		}
		copiedPath := args[1]
		cleanup()
		if _, err := os.Stat(copiedPath); !os.IsNotExist(err) {
			t.Errorf("temporary cookie copy still exists after cleanup: stat error = %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		t.Setenv("YTDLP_COOKIES_FILE", filepath.Join(t.TempDir(), "missing.txt"))

		if _, _, err := ytDlpCookiesArgs(); err == nil {
			t.Fatal("ytDlpCookiesArgs() error = nil, want missing-file error")
		}
	})

	t.Run("directory", func(t *testing.T) {
		t.Setenv("YTDLP_COOKIES_FILE", t.TempDir())

		if _, _, err := ytDlpCookiesArgs(); err == nil {
			t.Fatal("ytDlpCookiesArgs() error = nil, want regular-file error")
		}
	})
}

func TestInstagramYtDlpCookiesArgs(t *testing.T) {
	t.Run("uses Instagram cookie file when configured", func(t *testing.T) {
		instagramCookiesPath := filepath.Join(t.TempDir(), "instagram-cookies.txt")
		if err := os.WriteFile(instagramCookiesPath, []byte("# Instagram cookies\n"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("INSTAGRAM_COOKIES_FILE", instagramCookiesPath)
		t.Setenv("YTDLP_COOKIES_FILE", filepath.Join(t.TempDir(), "reddit-cookies.txt"))

		args, cleanup, err := instagramYtDlpCookiesArgs()
		if err != nil {
			t.Fatalf("instagramYtDlpCookiesArgs() error = %v", err)
		}
		defer cleanup()
		if len(args) != 2 || args[0] != "--cookies" {
			t.Fatalf("instagramYtDlpCookiesArgs() = %v, want --cookies and a file path", args)
		}
		got, err := os.ReadFile(args[1])
		if err != nil {
			t.Fatalf("read copied Instagram cookie file: %v", err)
		}
		if string(got) != "# Instagram cookies\n" {
			t.Fatalf("copied cookie content = %q, want Instagram cookie content", got)
		}
	})

	t.Run("falls back to generic cookie file", func(t *testing.T) {
		cookiesPath := filepath.Join(t.TempDir(), "cookies.txt")
		if err := os.WriteFile(cookiesPath, []byte("# Netscape HTTP Cookie File\n"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("INSTAGRAM_COOKIES_FILE", "")
		t.Setenv("YTDLP_COOKIES_FILE", cookiesPath)

		args, cleanup, err := instagramYtDlpCookiesArgs()
		if err != nil {
			t.Fatalf("instagramYtDlpCookiesArgs() error = %v", err)
		}
		defer cleanup()
		if len(args) != 2 || args[0] != "--cookies" {
			t.Fatalf("instagramYtDlpCookiesArgs() = %v, want --cookies and a file path", args)
		}
		got, err := os.ReadFile(args[1])
		if err != nil {
			t.Fatalf("read copied generic cookie file: %v", err)
		}
		if string(got) != "# Netscape HTTP Cookie File\n" {
			t.Fatalf("copied cookie content = %q, want generic cookie content", got)
		}
	})
}
