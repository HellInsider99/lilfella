package main

import (
	"strings"
	"testing"
)

func TestInstagramCaption(t *testing.T) {
	if got := instagramCaption("  \n "); got != "" {
		t.Errorf("instagramCaption(blank) = %q, want empty", got)
	}

	got := instagramCaption("Tom & <Jerry> 🐭")
	want := "<blockquote expandable>Tom &amp; &lt;Jerry&gt; 🐭</blockquote>"
	if got != want {
		t.Errorf("instagramCaption() = %q, want %q", got, want)
	}
}

func TestTruncateUTF16(t *testing.T) {
	if got := truncateUTF16("short", 10); got != "short" {
		t.Errorf("truncateUTF16(short) = %q, want unchanged", got)
	}

	// Each emoji is 2 UTF-16 units: 6 emojis = 12 units > 9.
	got := truncateUTF16("🐭🐭🐭🐭🐭🐭", 9)
	if got != "🐭🐭🐭🐭…" {
		t.Errorf("truncateUTF16(emojis) = %q, want %q", got, "🐭🐭🐭🐭…")
	}

	long := strings.Repeat("a", 2000)
	got = truncateUTF16(long, telegramCaptionLimit)
	if n := len([]rune(got)); n != telegramCaptionLimit {
		t.Errorf("truncateUTF16(long) has %d runes, want %d", n, telegramCaptionLimit)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncateUTF16(long) = %q..., want trailing ellipsis", got[:10])
	}
}
