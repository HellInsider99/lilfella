package main

type Track struct {
	Title          string
	Artist         string
	DurationMs     int
	SourcePlatform string
}

type Platform interface {
	Name() string
	ExtractURL(text string) (string, bool)
	TrackFromURL(trackURL string) (Track, error)
	SearchTrack(track Track) (string, error)
}
