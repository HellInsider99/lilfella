package downloader

import "testing"

func TestValidateRedditURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "post", url: "https://www.reddit.com/r/golang/comments/abc123/example/"},
		{name: "gallery", url: "https://www.reddit.com/r/golang/gallery/abc123"},
		{name: "short", url: "https://redd.it/abc123"},
		{name: "old host", url: "https://old.reddit.com/r/golang/comments/abc123/example/"},
		{name: "untrusted host", url: "https://reddit.com.example.org/r/golang/comments/abc123/", wantErr: true},
		{name: "unsupported scheme", url: "ftp://reddit.com/r/golang/comments/abc123/", wantErr: true},
		{name: "empty short", url: "https://redd.it/", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRedditURL(test.url)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateRedditURL(%q) error = %v, wantErr %t", test.url, err, test.wantErr)
			}
		})
	}
}

func TestParseRedditMediaGallery(t *testing.T) {
	output := []byte(`{"entries":[
		{"url":"https://preview.redd.it/one.jpg?width=1080&format=pjpg","ext":"jpg"},
		{"url":"https://preview.redd.it/two.jpg?width=1080&format=pjpg","ext":"jpg"}
	]}`)
	items, _, err := parseRedditYTDlpMedia(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("parseRedditMedia() returned %d items, want 2", len(items))
	}
	if items[0].Type != "image" || items[0].URL != "https://preview.redd.it/one.jpg?width=1080&format=pjpg" {
		t.Errorf("first item = %#v", items[0])
	}
}

func TestParseRedditMediaVideo(t *testing.T) {
	items, _, err := parseRedditYTDlpMedia([]byte(`{"url":"https://v.redd.it/video/DASH_720.mp4","ext":"mp4","vcodec":"h264"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Type != "video" {
		t.Fatalf("parseRedditMedia() = %#v, want video item", items)
	}
}

func TestParseRedditMediaVideoFormats(t *testing.T) {
	output := []byte(`{"formats":[
		{"url":"https://v.redd.it/video/audio","ext":"m4a","vcodec":"none","acodec":"aac"},
		{"url":"https://v.redd.it/video/DASH_720.mp4","ext":"mp4","vcodec":"h264"}
	]}`)
	items, _, err := parseRedditYTDlpMedia(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Type != "video" {
		t.Fatalf("parseRedditYTDlpMedia() = %#v, want one video item", items)
	}
}

func TestRedditImageFromUnsupportedMediaURL(t *testing.T) {
	output := []byte("ERROR: Unsupported URL : https://www.reddit.com/media?url=https%3A%2F%2Fpreview.redd.it%2Fimage.jpeg%3Fwidth%3D1080\n")
	item, ok := redditImageFromUnsupportedURL(output)
	if !ok {
		t.Fatal("redditImageFromUnsupportedURL() found no image")
	}
	if item.Type != "image" || item.URL != "https://preview.redd.it/image.jpeg?width=1080" {
		t.Errorf("redditImageFromUnsupportedURL() = %#v", item)
	}
}

func TestRedditImageFromUnsupportedURLRejectsUntrustedHosts(t *testing.T) {
	output := []byte("ERROR: Unsupported URL: https://www.reddit.com/media?url=https%3A%2F%2Fevil.example%2Fimage.jpeg\n")
	if _, ok := redditImageFromUnsupportedURL(output); ok {
		t.Fatal("redditImageFromUnsupportedURL() accepted untrusted media host")
	}
}

func TestParseRedditMediaText(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{name: "title only", output: `{"title":"Cool pic","url":"https://i.redd.it/a.jpg","ext":"jpg"}`, want: "Cool pic"},
		{name: "title and body", output: `{"title":"Cool pic","description":"Taken today","url":"https://i.redd.it/a.jpg","ext":"jpg"}`, want: "Cool pic\n\nTaken today"},
		{name: "body repeats title", output: `{"title":"Same","description":"Same","url":"https://i.redd.it/a.jpg","ext":"jpg"}`, want: "Same"},
		{name: "no text", output: `{"url":"https://i.redd.it/a.jpg","ext":"jpg"}`, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, text, err := parseRedditYTDlpMedia([]byte(test.output))
			if err != nil {
				t.Fatal(err)
			}
			if text != test.want {
				t.Errorf("text = %q, want %q", text, test.want)
			}
		})
	}
}
