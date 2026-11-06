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
	twitterURLPattern   = regexp.MustCompile(`https?://(?:www\.)?(?:x\.com|twitter\.com|vxtwitter\.com)/[^\s]+`)
	instagramURLPattern = regexp.MustCompile(`https?://(?:(?:www|m)\.)?instagram\.com/(?:reel|p)/[^\s]+`)
	shortsURLPattern    = regexp.MustCompile(`https?://(?:www\.)?(?:youtube\.com/shorts|music\.youtube\.com/shorts)/[^\s?]+`)
)

func main() {
	if err := loadEnvFile(".env"); err != nil {
		log.Fatal(err)
	}

	logFile, err := initLogging()
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

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
		log.Printf("incoming message: update_id=%d chat_id=%d message_id=%d chat_type=%s", update.UpdateID, update.Message.Chat.ID, update.Message.MessageID, update.Message.Chat.Type)
		if update.Message.Sticker != nil {
			if !update.Message.Chat.IsPrivate() {
				continue
			}
			log.Printf("sticker processing started: chat_id=%d message_id=%d", update.Message.Chat.ID, update.Message.MessageID)
			handleSticker(bot, update.Message)
			continue
		}
		if mediaURL, mediaType, ok := findDownloadableMediaInMessage(update.Message); ok {
			log.Printf("downloadable media detected: type=%s chat_id=%d message_id=%d", mediaType, update.Message.Chat.ID, update.Message.MessageID)
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

func findDownloadableMediaInMessage(message *tgbotapi.Message) (string, string, bool) {
	if message == nil {
		return "", "", false
	}

	for _, text := range []string{message.Text, message.Caption} {
		if mediaURL, mediaType, ok := findDownloadableMedia(text); ok {
			return mediaURL, mediaType, true
		}
	}

	for _, entities := range [][]tgbotapi.MessageEntity{message.Entities, message.CaptionEntities} {
		for _, entity := range entities {
			if entity.Type != "text_link" || entity.URL == "" {
				continue
			}
			if mediaURL, mediaType, ok := findDownloadableMedia(entity.URL); ok {
				return mediaURL, mediaType, true
			}
		}
	}

	return "", "", false
}

func handleDownloadableMedia(bot *tgbotapi.BotAPI, message *tgbotapi.Message, rawURL, mediaType string) {
	log.Printf("media download started: type=%s chat_id=%d message_id=%d", mediaType, message.Chat.ID, message.MessageID)
	status := tgbotapi.NewMessage(message.Chat.ID, "⏳ Посилання отримано. Починаю завантаження відео...")
	if message.Chat.Type == "group" || message.Chat.Type == "supergroup" {
		status.ReplyToMessageID = message.MessageID
	}
	statusMessage, err := bot.Send(status)
	if err != nil {
		log.Printf("failed to send media status: type=%s chat_id=%d message_id=%d: %v", mediaType, message.Chat.ID, message.MessageID, err)
		return
	}
	failStatus := func() {
		editStatus(bot, message.Chat.ID, statusMessage.MessageID, "❌ Не вдалося завантажити або надіслати відео 😕")
	}

	switch mediaType {
	case "twitter":
		directURL, err := downloader.TwitterVideoURL(rawURL)
		if err != nil {
			log.Printf("Twitter media resolution failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			failStatus()
			return
		}
		video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FileURL(directURL))
		video.ReplyToMessageID = message.MessageID
		if _, err := bot.Send(video); err != nil {
			log.Printf("Twitter video send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			failStatus()
			return
		}
		log.Printf("Twitter video sent: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
		deleteMessage(bot, message.Chat.ID, statusMessage.MessageID)
	case "instagram":
		filePath, cleanup, err := downloader.InstagramReel(rawURL)
		if err != nil {
			log.Printf("Instagram download failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			failStatus()
			return
		}
		defer cleanup()
		video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FilePath(filePath))
		video.ReplyToMessageID = message.MessageID
		if _, err := bot.Send(video); err != nil {
			log.Printf("Instagram video send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			failStatus()
			return
		}
		log.Printf("Instagram video sent: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
		deleteMessage(bot, message.Chat.ID, statusMessage.MessageID)
	case "youtube-video":
		filePath, cleanup, err := downloader.YouTubeVideo(rawURL)
		if err != nil {
			log.Printf("YouTube video download failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			failStatus()
			return
		}
		defer cleanup()
		video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FilePath(filePath))
		video.ReplyToMessageID = message.MessageID
		if _, err := bot.Send(video); err != nil {
			log.Printf("YouTube video send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			failStatus()
			return
		}
		log.Printf("YouTube video sent: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
		deleteMessage(bot, message.Chat.ID, statusMessage.MessageID)
	default:
		log.Printf("unsupported media type: type=%q chat_id=%d message_id=%d", mediaType, message.Chat.ID, message.MessageID)
		failStatus()
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
	log.Printf("music link processing started: chat_id=%d message_id=%d group=%t", message.Chat.ID, message.MessageID, isGroup)
	source, ok := findSourcePlatform(message.Text, platforms)
	if !ok {
		log.Printf("no supported music link found: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
		if !isGroup {
			sendReply(bot, message, "❌ Не знайшов підтримуваного музичного посилання 😕")
		}
		return
	}
	log.Printf("music source identified: platform=%q chat_id=%d message_id=%d", source.Name(), message.Chat.ID, message.MessageID)

	sourceURL, _ := source.ExtractURL(message.Text)
	status := tgbotapi.NewMessage(message.Chat.ID, "⏳ Посилання отримано. Визначаю трек...")
	if isGroup {
		status.ReplyToMessageID = message.MessageID
	}
	statusMessage, err := bot.Send(status)
	if err != nil {
		log.Printf("failed to send status message: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
		return
	}

	log.Printf("track lookup started: platform=%q chat_id=%d message_id=%d", source.Name(), message.Chat.ID, message.MessageID)
	track, err := source.TrackFromURL(sourceURL)
	if err != nil {
		log.Printf("track lookup failed: platform=%q chat_id=%d message_id=%d: %v", source.Name(), message.Chat.ID, message.MessageID, err)
		editStatus(bot, message.Chat.ID, statusMessage.MessageID, "❌ Не вдалося прочитати трек 😕")
		return
	}
	log.Printf("track identified: platform=%q artist=%q title=%q chat_id=%d message_id=%d", source.Name(), track.Artist, track.Title, message.Chat.ID, message.MessageID)
	cleanTitle := CleanTrackTitle(track.Title)
	cleanArtist := sanitizeSearchArtist(CleanArtistName(track.Artist))
	if cleanTitle == "" {
		log.Printf("track title is empty after cleanup: platform=%q chat_id=%d message_id=%d", source.Name(), message.Chat.ID, message.MessageID)
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
		log.Printf("platform search started: target=%q artist=%q title=%q chat_id=%d message_id=%d", target.Name(), searchTrack.Artist, searchTrack.Title, message.Chat.ID, message.MessageID)
		targetURL, searchErr := target.SearchTrack(searchTrack)
		if searchErr == nil {
			log.Printf("platform search succeeded: target=%q chat_id=%d message_id=%d", target.Name(), message.Chat.ID, message.MessageID)
			links = append(links, fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(targetURL), html.EscapeString(target.Name())))
			if target.Name() == "YouTube Music" {
				youtubeAudioURL = targetURL
			}
		} else {
			log.Printf("platform search failed: target=%q artist=%q title=%q chat_id=%d message_id=%d: %v", target.Name(), searchTrack.Artist, searchTrack.Title, message.Chat.ID, message.MessageID, searchErr)
		}
	}

	if len(links) == 0 {
		log.Printf("no platform matches found: artist=%q title=%q chat_id=%d message_id=%d", track.Artist, track.Title, message.Chat.ID, message.MessageID)
		if isGroup {
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
		log.Printf("YouTube audio URL selected: source=%q chat_id=%d message_id=%d", source.Name(), message.Chat.ID, message.MessageID)
		editStatus(bot, message.Chat.ID, statusMessage.MessageID, "⏳ Завантажую аудіо...")
		if err := sendYouTubeAudio(bot, message, youtubeAudioURL, result); err == nil {
			log.Printf("YouTube audio workflow completed: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
			deleteMessage(bot, message.Chat.ID, statusMessage.MessageID)
			return
		} else {
			log.Printf("YouTube audio workflow failed; sending text fallback: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
		}
	} else {
		log.Printf("YouTube audio skipped: no YouTube Music result available: source=%q chat_id=%d message_id=%d", source.Name(), message.Chat.ID, message.MessageID)
	}

	editStatusHTML(bot, message.Chat.ID, statusMessage.MessageID, result)
	log.Printf("music link processing completed with text result: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
}

func sendYouTubeAudio(bot *tgbotapi.BotAPI, message *tgbotapi.Message, youtubeURL string, caption string) error {
	log.Printf("YouTube audio download started: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
	filePath, cleanup, err := downloader.YouTubeAudio(youtubeURL)
	if err != nil {
		return fmt.Errorf("YouTube audio download failed: %w", err)
	}
	defer cleanup()
	log.Printf("YouTube audio download completed: path=%q chat_id=%d message_id=%d", filePath, message.Chat.ID, message.MessageID)

	audio := tgbotapi.NewAudio(message.Chat.ID, tgbotapi.FilePath(filePath))
	audio.ReplyToMessageID = message.MessageID
	audio.Caption = caption
	audio.ParseMode = tgbotapi.ModeHTML
	log.Printf("YouTube audio upload started: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
	if _, err := bot.Send(audio); err != nil {
		return fmt.Errorf("failed to send YouTube audio: %w", err)
	}
	log.Printf("YouTube audio upload completed: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
	return nil
}

func sendReply(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text string) {
	if _, err := bot.Send(tgbotapi.NewMessage(message.Chat.ID, text)); err != nil {
		log.Printf("failed to send reply: %v", err)
		return
	}
	log.Printf("reply sent: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
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
		return
	}
	log.Printf("HTML status message edited: chat_id=%d status_message_id=%d", chatID, messageID)
}

func deleteMessage(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	if _, err := bot.Send(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
		log.Printf("failed to delete message: %v", err)
		return
	}
	log.Printf("status message deleted: chat_id=%d status_message_id=%d", chatID, messageID)
}
