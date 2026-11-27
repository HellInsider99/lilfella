package main

import "testing"

func TestParseStatusChatList(t *testing.T) {
	list, err := parseStatusChatList([]byte(`[123456789, -1001234567890]`))
	if err != nil {
		t.Fatal(err)
	}
	if !list.Contains(123456789) || !list.Contains(-1001234567890) {
		t.Errorf("list = %v, want both chat IDs", list)
	}
	if list.Contains(42) {
		t.Error("list.Contains(42) = true, want false")
	}

	empty, err := parseStatusChatList([]byte(`[]`))
	if err != nil || len(empty) != 0 {
		t.Errorf("parseStatusChatList([]) = %v, %v; want empty list", empty, err)
	}

	for _, bad := range []string{`{"ids": [1]}`, `["123"]`, `not json`} {
		if _, err := parseStatusChatList([]byte(bad)); err == nil {
			t.Errorf("parseStatusChatList(%s) error = nil, want error", bad)
		}
	}
}
