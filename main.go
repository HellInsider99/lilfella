package main

import (
	"fmt"
	"html"
	"log"
	"os"
	"regexp"
	"strings"

	"deezer-bot/downloader"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	twitterURLPattern   = regexp.MustCompile(`https?://(?:www\.)?(?:x\.com|twitter\.com)/[^\s]+`)
	instagramURLPattern = regexp.MustCompile(`https?://(?:www\.)?instagram\.com/(?:reel|p)/[^\s]+`)
	shortsURLPattern    = regexp.MustCompile(`https?://(?:www\.)?(?:youtube\.com/shorts|music\.youtube\.com/shorts)/[^\s?]+`)
)

func main() {
	if err := loadEnvFile(".env"); err != nil {
		log.Fatal(err)
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	platforms := []Platform{
		NewSpotifyPlatform(os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET")),
		NewDeezerPlatform(),
		NewYouTubePlatform(os.Getenv("YOUTUBE_API_KEY")),
		NewAppleMusicPlatform(),
	}

	log.Printf("Bot started as @%s", bot.Self.UserName)

	updates := bot.GetUpdatesChan(tgbotapi.UpdateConfig{Offset: 0, Timeout: 60})
	for update := range updates {
		if update.Message == nil {
			continue
		}
		if update.Message.Sticker != nil {
			if !update.Message.Chat.IsPrivate() {
				continue
			}
			handleSticker(bot, update.Message)
			continue
		}
		if mediaURL, mediaType, ok := findDownloadableMedia(update.Message.Text); ok {
			if update.Message.Chat.IsPrivate() {
				handleDownloadableMedia(bot, update.Message, mediaURL, mediaType)
			} else if update.Message.Chat.Type == "group" || update.Message.Chat.Type == "supergroup" {
				go handleDownloadableMedia(bot, update.Message, mediaURL, mediaType)
			}
			continue
		}

		switch update.Message.Chat.Type {
		case "group", "supergroup":
			if _, ok := findSourcePlatform(update.Message.Text, platforms); !ok {
				continue
			}
			go handleMessage(bot, update.Message, platforms, true)
		case "private":
			handleMessage(bot, update.Message, platforms, false)
		}
	}
}

func findDownloadableMedia(text string) (string, string, bool) {
	if match := twitterURLPattern.FindString(text); match != "" {
		return strings.TrimRight(match, ".,!?)]}"), "twitter", true
	}
	if match := instagramURLPattern.FindString(text); match != "" {
		return strings.TrimRight(match, ".,!?)]}"), "instagram", true
	}
	if match := shortsURLPattern.FindString(text); match != "" {
		return strings.TrimRight(match, ".,!?)]}"), "youtube-video", true
	}
	return "", "", false
}

func handleDownloadableMedia(bot *tgbotapi.BotAPI, message *tgbotapi.Message, rawURL, mediaType string) {
	switch mediaType {
	case "twitter":
		directURL, err := downloader.TwitterVideoURL(rawURL)
		if err != nil {
			log.Printf("Twitter download failed: %v", err)
			return
		}
		video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FileURL(directURL))
		video.ReplyToMessageID = message.MessageID
		if _, err := bot.Send(video); err != nil {
			log.Printf("failed to send Twitter video: %v", err)
		}
	case "instagram":
		filePath, cleanup, err := downloader.InstagramReel(rawURL)
		if err != nil {
			log.Printf("Instagram download failed: %v", err)
			return
		}
		defer cleanup()
		video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FilePath(filePath))
		video.ReplyToMessageID = message.MessageID
		if _, err := bot.Send(video); err != nil {
			log.Printf("failed to send Instagram video: %v", err)
		}
	case "youtube-video":
		filePath, cleanup, err := downloader.YouTubeVideo(rawURL)
		if err != nil {
			log.Printf("YouTube video download failed: %v", err)
			return
		}
		defer cleanup()
		video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FilePath(filePath))
		video.ReplyToMessageID = message.MessageID
		if _, err := bot.Send(video); err != nil {
			log.Printf("failed to send YouTube video: %v", err)
		}
	}
}

func findSourcePlatform(text string, platforms []Platform) (Platform, bool) {
	for _, platform := range platforms {
		if _, ok := platform.ExtractURL(text); ok {
			return platform, true
		}
	}
	return nil, false
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, platforms []Platform, isGroup bool) {
	source, ok := findSourcePlatform(message.Text, platforms)
	if !ok {
		if !isGroup {
			sendReply(bot, message, "❌ Не знайшов підтримуваного музичного посилання 😕")
		}
		return
	}

	sourceURL, _ := source.ExtractURL(message.Text)
	status := tgbotapi.NewMessage(message.Chat.ID, "⏳ Посилання отримано. Визначаю трек...")
	if isGroup {
		status.ReplyToMessageID = message.MessageID
	}
	statusMessage, err := bot.Send(status)
	if err != nil {
		log.Printf("failed to send status message: %v", err)
		return
	}

	track, err := source.TrackFromURL(sourceURL)
	if err != nil {
		if source.Name() == "YouTube Music" {
			log.Printf("Error parsing YouTube link: %v", err)
		} else {
			log.Printf("Error parsing %s link: %v", source.Name(), err)
		}
		editStatus(bot, message.Chat.ID, statusMessage.MessageID, "❌ Не вдалося прочитати трек 😕")
		return
	}
	cleanTitle := CleanTrackTitle(track.Title)
	cleanArtist := sanitizeSearchArtist(CleanArtistName(track.Artist))
	if cleanTitle == "" {
		editStatus(bot, message.Chat.ID, statusMessage.MessageID, "❌ Не вдалося визначити назву або автора треку 😕")
		return
	}
	searchTrack := track
	searchTrack.Title = cleanTitle
	searchTrack.Artist = cleanArtist
	searchTrack.SourcePlatform = source.Name()

	links := make([]string, 0, len(platforms)-1)
	youtubeAudioURL := ""
	for _, target := range platforms {
		if target == source {
			continue
		}

		editStatus(bot, message.Chat.ID, statusMessage.MessageID, fmt.Sprintf("⏳ Шукаю трек у %s...", target.Name()))
		targetURL, searchErr := target.SearchTrack(searchTrack)
		if searchErr == nil {
			links = append(links, fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(targetURL), html.EscapeString(target.Name())))
			if target.Name() == "YouTube Music" {
				youtubeAudioURL = targetURL
			}
		} else {
			log.Printf("%s search failed for %q by %q: %v", target.Name(), searchTrack.Title, searchTrack.Artist, searchErr)
		}
	}

	if len(links) == 0 {
		if isGroup {
			log.Printf("No platform match for %q by %q", track.Title, track.Artist)
			editStatus(bot, message.Chat.ID, statusMessage.MessageID, "❌ Не вдалося знайти трек на інших платформах")
		} else {
			editStatus(bot, message.Chat.ID, statusMessage.MessageID, fmt.Sprintf("❌ Не вдалося знайти трек на інших платформах: %s", strings.TrimSpace(track.Title)))
		}
		return
	}

	links = append(links, fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(sourceURL), html.EscapeString(source.Name())))
	result := fmt.Sprintf("✅ <b>%s - %s</b>\n%s", html.EscapeString(strings.TrimSpace(track.Artist)), html.EscapeString(strings.TrimSpace(track.Title)), strings.Join(links, " | "))

	if source.Name() == "YouTube Music" {
		youtubeAudioURL = sourceURL
	}

	if youtubeAudioURL != "" {
		editStatus(bot, message.Chat.ID, statusMessage.MessageID, "⏳ Завантажую аудіо...")
		if err := sendYouTubeAudio(bot, message, youtubeAudioURL, result); err == nil {
			deleteMessage(bot, message.Chat.ID, statusMessage.MessageID)
			return
		} else {
			log.Printf("audio sending failed, falling back to text: %v", err)
		}
	}

	editStatusHTML(bot, message.Chat.ID, statusMessage.MessageID, result)
}

func sendYouTubeAudio(bot *tgbotapi.BotAPI, message *tgbotapi.Message, youtubeURL string, caption string) error {
	filePath, cleanup, err := downloader.YouTubeAudio(youtubeURL)
	if err != nil {
		return fmt.Errorf("YouTube audio download failed: %w", err)
	}
	defer cleanup()

	audio := tgbotapi.NewAudio(message.Chat.ID, tgbotapi.FilePath(filePath))
	audio.ReplyToMessageID = message.MessageID
	audio.Caption = caption
	audio.ParseMode = tgbotapi.ModeHTML
	if _, err := bot.Send(audio); err != nil {
		return fmt.Errorf("failed to send YouTube audio: %w", err)
	}
	return nil
}

func sendReply(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text string) {
	if _, err := bot.Send(tgbotapi.NewMessage(message.Chat.ID, text)); err != nil {
		log.Printf("failed to send reply: %v", err)
	}
}

func editStatus(bot *tgbotapi.BotAPI, chatID int64, messageID int, text string) {
	if messageID == 0 {
		return
	}
	if _, err := bot.Send(tgbotapi.NewEditMessageText(chatID, messageID, text)); err != nil {
		log.Printf("failed to edit status message: %v", err)
	}
}

func editStatusHTML(bot *tgbotapi.BotAPI, chatID int64, messageID int, text string) {
	if messageID == 0 {
		return
	}

	message := tgbotapi.NewEditMessageText(chatID, messageID, text)
	message.ParseMode = tgbotapi.ModeHTML
	message.DisableWebPagePreview = true
	if _, err := bot.Send(message); err != nil {
		log.Printf("failed to edit HTML status message: %v", err)
	}
}

func deleteMessage(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
		log.Printf("failed to delete message: %v", err)
	}
}
