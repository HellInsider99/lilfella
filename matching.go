package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type TrackResult struct {
	Title      string
	Artist     string
	DurationMs int
	URL        string
}

const durationToleranceMs = 10_000

var (
	trackFeatureGroupPattern  = regexp.MustCompile(`(?i)\s*[\[(](?:feat|ft|with|prod)\.?\s+[^\])]+[\])]`)
	trackFeatureSuffixPattern = regexp.MustCompile(`(?i)\s+-\s+(?:feat|ft|with|prod)\.?\s+.*$`)
	trackNoiseGroupPattern    = regexp.MustCompile(`(?i)\s*(?:\((?:official|video|audio|lyrics|topic|vevo)[^)]*\)|\[(?:official|video|audio|lyrics|topic|vevo)[^\]]*\])`)
	trackNoisePattern         = regexp.MustCompile(`(?i)\b(?:official\s+video|music\s+video|lyrics|audio|topic)\b`)
	artistNoisePattern        = regexp.MustCompile(`(?i)(?:\s*-\s*topic\b|\btopic\b|\bvevo\b|\bofficial\b)`)
	artistDelimiterPattern    = regexp.MustCompile(`(?i)\s*(?:&|,|\band\b|\bx\b|\bvs\.?|\bfeat\.?|\bft\.?)\s*`)
)

func CleanTrackTitle(title string) string {
	cleaned := trackFeatureGroupPattern.ReplaceAllString(title, " ")
	cleaned = trackFeatureSuffixPattern.ReplaceAllString(cleaned, " ")
	cleaned = trackNoiseGroupPattern.ReplaceAllString(cleaned, " ")
	cleaned = trackNoisePattern.ReplaceAllString(cleaned, " ")
	return strings.ToLower(strings.Join(strings.Fields(cleaned), " "))
}

func CleanArtistName(artist string) string {
	cleaned := trackNoiseGroupPattern.ReplaceAllString(artist, " ")
	cleaned = artistNoisePattern.ReplaceAllString(cleaned, " ")
	return strings.ToLower(strings.Join(strings.Fields(cleaned), " "))
}

func sanitizeSearchArtist(artist string) string {
	cleaned := strings.ToLower(strings.TrimSpace(artist))
	switch cleaned {
	case "release", "topic", "release - topic", "various artists":
		return ""
	default:
		return artist
	}
}

func splitCollaboratingArtists(artist string) []string {
	normalized := artistDelimiterPattern.ReplaceAllString(artist, ", ")
	parts := strings.Split(normalized, ",")
	artists := make([]string, 0, len(parts))
	for _, part := range parts {
		if cleaned := strings.TrimSpace(part); cleaned != "" {
			artists = append(artists, cleaned)
		}
	}
	return artists
}

func cleanCollaboratingArtists(artist string) []string {
	normalized := artistDelimiterPattern.ReplaceAllString(artist, ", ")
	parts := strings.Split(normalized, ",")
	artists := make([]string, 0, len(parts))
	for _, part := range parts {
		if cleaned := CleanArtistName(part); cleaned != "" {
			artists = append(artists, cleaned)
		}
	}
	return artists
}

func CalculateSimilarity(a, b string) float64 {
	if a == b {
		return 100.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	left := []rune(a)
	right := []rune(b)
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for column := range previous {
		previous[column] = column
	}

	for row := 1; row <= len(left); row++ {
		current[0] = row
		for column := 1; column <= len(right); column++ {
			cost := 0
			if left[row-1] != right[column-1] {
				cost = 1
			}
			current[column] = minInt(
				current[column-1]+1,
				previous[column]+1,
				previous[column-1]+cost,
			)
		}
		previous, current = current, previous
	}

	maxLength := utf8.RuneCountInString(a)
	if rightLength := utf8.RuneCountInString(b); rightLength > maxLength {
		maxLength = rightLength
	}
	return (1.0 - float64(previous[len(right)])/float64(maxLength)) * 100.0
}

func FindBestMatch(originalDurationMs int, originalCleanTitle, originalCleanArtist string, results []TrackResult) *TrackResult {
	originalArtists := cleanCollaboratingArtists(originalCleanArtist)
	isRemix := strings.Contains(strings.ToLower(originalCleanTitle), "remix")
	titleThreshold := 85.0
	if isRemix {
		titleThreshold = 60.0
	}

	for index := range results {
		cleanResultTitle := CleanTrackTitle(results[index].Title)
		titleSim := CalculateSimilarity(originalCleanTitle, cleanResultTitle)
		artistMatch := false
		artistSim := 0.0
		matchedOriginalArtist := ""
		matchedResultArtist := ""
		resultArtists := cleanCollaboratingArtists(results[index].Artist)
		for _, originalArtist := range originalArtists {
			for _, resultArtist := range resultArtists {
				candidateArtistSim := CalculateSimilarity(originalArtist, resultArtist)
				if candidateArtistSim > artistSim {
					artistSim = candidateArtistSim
					matchedOriginalArtist = originalArtist
					matchedResultArtist = resultArtist
				}
				if candidateArtistSim > 80.0 {
					artistMatch = true
				}
			}
		}
		fmt.Printf("Match Check -> Title: '%s' vs '%s' (Score: %.2f) | Artist: '%s' vs '%s' (Score: %.2f)\n", originalCleanTitle, cleanResultTitle, titleSim, matchedOriginalArtist, matchedResultArtist, artistSim)
		titleMatch := titleSim > titleThreshold
		if isRemix && strings.Contains(cleanResultTitle, originalCleanTitle) {
			titleMatch = true
		}
		if titleSim >= 90.0 && artistSim >= 90.0 && results[index].URL != "" {
			return &results[index]
		}

		durationDifference := originalDurationMs - results[index].DurationMs
		if durationDifference < 0 {
			durationDifference = -durationDifference
		}
		if durationDifference > durationToleranceMs {
			fmt.Printf("Match Check -> rejected duration: original %dms vs result %dms (difference: %dms), title: '%s', artist: '%s'\n", originalDurationMs, results[index].DurationMs, durationDifference, results[index].Title, results[index].Artist)
			continue
		}
		if titleMatch && artistMatch {
			return &results[index]
		}
	}
	return nil
}

func FindBestMatchForSource(originalDurationMs int, originalCleanTitle, originalCleanArtist, sourcePlatform string, results []TrackResult) *TrackResult {
	if strings.EqualFold(sourcePlatform, "YouTube Music") {
		for index := range results {
			result := &results[index]
			if CalculateSimilarity(originalCleanTitle, CleanTrackTitle(result.Title)) >= 75.0 && result.URL != "" {
				return result
			}
			durationDifference := originalDurationMs - result.DurationMs
			if durationDifference < 0 {
				durationDifference = -durationDifference
			}
			if originalDurationMs != 0 && result.DurationMs != 0 && durationDifference > durationToleranceMs {
				continue
			}
		}
	}
	return FindBestMatch(originalDurationMs, originalCleanTitle, originalCleanArtist, results)
}

func FindBestYouTubeMatch(originalDurationMs int, originalCleanTitle string, results []TrackResult) *TrackResult {
	if originalCleanTitle == "" {
		return nil
	}

	for index := range results {
		cleanResultTitle := CleanTrackTitle(results[index].Title)
		if strings.Contains(cleanResultTitle, originalCleanTitle) {
			return &results[index]
		}
		durationDifference := originalDurationMs - results[index].DurationMs
		if durationDifference < 0 {
			durationDifference = -durationDifference
		}
		if durationDifference > durationToleranceMs {
			continue
		}
	}
	return nil
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}
