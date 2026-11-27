package main

import (
	"fmt"
	"html"
	"log"
	"os"
	"regexp"
	"strings"
	"unicode"

	"deezer-bot/downloader"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	twitterURLPattern   = regexp.MustCompile(`https?://(?:www\.)?(?:x\.com|twitter\.com|vxtwitter\.com)/[^\s]+`)
	instagramURLPattern = regexp.MustCompile(`https?://(?:(?:www|m)\.)?instagram\.com/(?:reels?|p)/[^\s]+`)
	redditURLPattern    = regexp.MustCompile(`https?://(?:(?:www|old|new)\.)?reddit\.com/(?:r/[^/\s]+/(?:comments|s|gallery)/[^\s]+)[^\s]*|https?://redd\.it/[A-Za-z0-9]+[^\s]*`)
	shortsURLPattern    = regexp.MustCompile(`https?://(?:www\.)?(?:youtube\.com/shorts|music\.youtube\.com/shorts)/[^\s?]+`)
	tiktokURLPattern    = regexp.MustCompile(`https?://(?:(?:www|m)\.)?tiktok\.com/(?:@[^/\s]+/(?:video|photo)/\d+|t/[^/\s]+)[^\s]*|https?://(?:vm|vt)\.tiktok\.com/[^\s]+`)
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
	if match := redditURLPattern.FindString(text); match != "" {
		return strings.TrimRight(match, ".,!?)]}"), "reddit", true
	}
	if match := shortsURLPattern.FindString(text); match != "" {
		return strings.TrimRight(match, ".,!?)]}"), "youtube-video", true
	}
	if match := tiktokURLPattern.FindString(text); match != "" {
		return strings.TrimRight(match, ".,!?)]}"), "tiktok", true
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

// handleDownloadableMedia downloads and sends the media behind rawURL. It posts
// no status message: only the media is sent, and failures are only logged.
func handleDownloadableMedia(bot *tgbotapi.BotAPI, message *tgbotapi.Message, rawURL, mediaType string) {
	log.Printf("media download started: type=%s chat_id=%d message_id=%d", mediaType, message.Chat.ID, message.MessageID)

	switch mediaType {
	case "twitter":
		media, text, err := downloader.TwitterMedia(rawURL)
		if err != nil {
			log.Printf("Twitter media resolution failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			return
		}
		caption := &captionOnce{html: mediaCaption(text)}
		// Consecutive photos are collected and sent as albums, keeping post order.
		var pendingPhotos []string
		flushPhotos := func() bool {
			if len(pendingPhotos) == 0 {
				return true
			}
			err := sendPhotoURLs(bot, message, pendingPhotos, caption.take())
			pendingPhotos = nil
			if err != nil {
				log.Printf("Twitter photo send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
				return false
			}
			return true
		}
		for _, item := range media {
			switch item.Type {
			case "image":
				pendingPhotos = append(pendingPhotos, item.URL)
			case "video", "animated_gif":
				if !flushPhotos() {
					return
				}
				video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FileURL(item.URL))
				if captionHTML := caption.take(); captionHTML != "" {
					video.Caption = captionHTML
					video.ParseMode = tgbotapi.ModeHTML
				}
				video.ReplyToMessageID = message.MessageID
				video.DisableNotification = true
				video.SupportsStreaming = true
				if _, err := bot.Send(video); err != nil {
					log.Printf("Twitter video send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
					return
				}
			default:
				log.Printf("unsupported Twitter media type: type=%q url=%q chat_id=%d message_id=%d", item.Type, item.URL, message.Chat.ID, message.MessageID)
				return
			}
		}
		if !flushPhotos() {
			return
		}
		log.Printf("Twitter media sent: count=%d has_caption=%t chat_id=%d message_id=%d", len(media), text != "", message.Chat.ID, message.MessageID)
	case "instagram":
		post, cleanup, err := downloader.InstagramMedia(rawURL)
		if err != nil {
			log.Printf("Instagram download failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			return
		}
		defer cleanup()
		if err := sendMediaFiles(bot, message, post.Items, mediaCaption(post.Description)); err != nil {
			log.Printf("Instagram media send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			return
		}
		log.Printf("Instagram media sent: count=%d has_caption=%t chat_id=%d message_id=%d", len(post.Items), post.Description != "", message.Chat.ID, message.MessageID)
	case "reddit":
		media, text, err := downloader.RedditMedia(rawURL)
		if err != nil {
			log.Printf("Reddit media resolution failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			return
		}
		caption := &captionOnce{html: mediaCaption(text)}
		// Consecutive photos are collected and sent as albums, keeping post order.
		var pendingPhotos []string
		flushPhotos := func() bool {
			if len(pendingPhotos) == 0 {
				return true
			}
			err := sendPhotoURLs(bot, message, pendingPhotos, caption.take())
			pendingPhotos = nil
			if err != nil {
				log.Printf("Reddit photo send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
				return false
			}
			return true
		}
		for _, item := range media {
			if item.Type == "image" {
				pendingPhotos = append(pendingPhotos, item.URL)
				continue
			}
			if !flushPhotos() {
				return
			}

			filePath, cleanup, err := downloader.RedditVideo(rawURL)
			if err != nil {
				log.Printf("Reddit video download failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
				return
			}
			err = sendLocalVideo(bot, message, filePath, caption.take())
			cleanup()
			if err != nil {
				log.Printf("Reddit video send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
				return
			}
		}
		if !flushPhotos() {
			return
		}
		log.Printf("Reddit media sent: count=%d has_caption=%t chat_id=%d message_id=%d", len(media), text != "", message.Chat.ID, message.MessageID)
	case "youtube-video":
		filePath, title, cleanup, err := downloader.YouTubeVideo(rawURL)
		if err != nil {
			log.Printf("YouTube video download failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			return
		}
		defer cleanup()
		if err := sendLocalVideo(bot, message, filePath, mediaCaption(title)); err != nil {
			log.Printf("YouTube video send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			return
		}
		log.Printf("YouTube video sent: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
	case "tiktok":
		if !downloader.IsTikTokPhotoURL(rawURL) {
			filePath, description, cleanup, err := downloader.TikTokVideo(rawURL)
			if err == nil {
				defer cleanup()
				if err := sendLocalVideo(bot, message, filePath, mediaCaption(description)); err != nil {
					log.Printf("TikTok video send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
					return
				}
				log.Printf("TikTok video sent: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
				return
			}
			// Short links can lead to photo posts, which yt-dlp can't download.
			log.Printf("TikTok video download failed; trying photo post: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
		}

		photos, description, cleanup, err := downloader.TikTokPhotos(rawURL)
		if err != nil {
			log.Printf("TikTok download failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			return
		}
		defer cleanup()
		if err := sendMediaFiles(bot, message, photos, mediaCaption(description)); err != nil {
			log.Printf("TikTok photo send failed: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
			return
		}
		log.Printf("TikTok photos sent: count=%d chat_id=%d message_id=%d", len(photos), message.Chat.ID, message.MessageID)
	default:
		log.Printf("unsupported media type: type=%q chat_id=%d message_id=%d", mediaType, message.Chat.ID, message.MessageID)
	}
}

// sendLocalVideo sends a local video; captionHTML, if not empty, is sent with
// HTML parse mode.
func sendLocalVideo(bot *tgbotapi.BotAPI, message *tgbotapi.Message, filePath, captionHTML string) error {
	video := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FilePath(filePath))
	if captionHTML != "" {
		video.Caption = captionHTML
		video.ParseMode = tgbotapi.ModeHTML
	}
	video.ReplyToMessageID = message.MessageID
	video.DisableNotification = true
	video.SupportsStreaming = true

	thumbnailPath, cleanupThumbnail, err := downloader.VideoThumbnail(filePath)
	if err != nil {
		log.Printf("video thumbnail generation failed; sending without custom thumbnail: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
	} else {
		defer cleanupThumbnail()
		video.Thumb = tgbotapi.FilePath(thumbnailPath)
	}

	_, err = bot.Send(video)
	return err
}

// telegramMediaGroupLimit is the maximum number of items Telegram accepts in one album.
const telegramMediaGroupLimit = 10

// telegramCaptionLimit is the maximum caption length Telegram accepts, in
// UTF-16 code units, counted after HTML tags are removed.
const telegramCaptionLimit = 1024

// mediaCaption formats post text as an HTML expandable quote for a Telegram
// caption, trimming it to fit telegramCaptionLimit. It returns "" for empty text.
func mediaCaption(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	return "<blockquote expandable>" + html.EscapeString(truncateUTF16(description, telegramCaptionLimit)) + "</blockquote>"
}

// truncateUTF16 shortens text to at most limit UTF-16 code units (how Telegram
// counts length), ending with "…" when it had to cut. It never splits a rune.
func truncateUTF16(text string, limit int) string {
	length := 0
	for _, r := range text {
		length += utf16RuneLen(r)
	}
	if length <= limit {
		return text
	}

	const ellipsis = "…"
	budget := limit - utf16RuneLen([]rune(ellipsis)[0])
	used := 0
	for i, r := range text {
		if used+utf16RuneLen(r) > budget {
			return strings.TrimRightFunc(text[:i], unicode.IsSpace) + ellipsis
		}
		used += utf16RuneLen(r)
	}
	return text
}

func utf16RuneLen(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// captionOnce hands a caption to the first message sent for a post only, so
// posts split into several messages show the text once, under the first.
type captionOnce struct {
	html string
}

func (c *captionOnce) take() string {
	captionHTML := c.html
	c.html = ""
	return captionHTML
}

// sendPhotoURLs sends photos by URL in order, grouped into albums of up to
// telegramMediaGroupLimit. A group with a single photo (an album needs at least
// two) is sent as a regular photo message. An empty list sends nothing.
// captionHTML, if not empty, goes under the first message.
func sendPhotoURLs(bot *tgbotapi.BotAPI, message *tgbotapi.Message, urls []string, captionHTML string) error {
	for groupIndex, group := range chunkSlice(urls, telegramMediaGroupLimit) {
		caption := ""
		if groupIndex == 0 {
			caption = captionHTML
		}
		if len(group) == 1 {
			photo := tgbotapi.NewPhoto(message.Chat.ID, tgbotapi.FileURL(group[0]))
			if caption != "" {
				photo.Caption = caption
				photo.ParseMode = tgbotapi.ModeHTML
			}
			photo.ReplyToMessageID = message.MessageID
			photo.DisableNotification = true
			if _, err := bot.Send(photo); err != nil {
				return fmt.Errorf("send photo: %w", err)
			}
			continue
		}

		files := make([]interface{}, 0, len(group))
		for i, photoURL := range group {
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FileURL(photoURL))
			if i == 0 && caption != "" {
				photo.Caption = caption
				photo.ParseMode = tgbotapi.ModeHTML
			}
			files = append(files, photo)
		}
		album := tgbotapi.NewMediaGroup(message.Chat.ID, files)
		album.ReplyToMessageID = message.MessageID
		album.DisableNotification = true
		if _, err := bot.SendMediaGroup(album); err != nil {
			return fmt.Errorf("send album of %d photos: %w", len(group), err)
		}
	}
	return nil
}

// sendMediaFiles sends downloaded media in post order, grouped into albums of up
// to telegramMediaGroupLimit items. A group with a single item (an album needs
// at least two) is sent as a regular photo or video message. captionHTML, if
// not empty, is attached to the first item, so it shows under the first message.
func sendMediaFiles(bot *tgbotapi.BotAPI, message *tgbotapi.Message, media []downloader.MediaFile, captionHTML string) error {
	for groupIndex, group := range chunkSlice(media, telegramMediaGroupLimit) {
		caption := ""
		if groupIndex == 0 {
			caption = captionHTML
		}
		if len(group) == 1 {
			item := group[0]
			if item.Type == "image" {
				photo := tgbotapi.NewPhoto(message.Chat.ID, tgbotapi.FilePath(item.Path))
				if caption != "" {
					photo.Caption = caption
					photo.ParseMode = tgbotapi.ModeHTML
				}
				photo.ReplyToMessageID = message.MessageID
				photo.DisableNotification = true
				if _, err := bot.Send(photo); err != nil {
					return fmt.Errorf("send photo: %w", err)
				}
				continue
			}
			if err := sendLocalVideo(bot, message, item.Path, caption); err != nil {
				return fmt.Errorf("send video: %w", err)
			}
			continue
		}

		if err := sendMediaFileAlbum(bot, message, group, caption); err != nil {
			return err
		}
	}
	return nil
}

// sendMediaFileAlbum sends group as one album. captionHTML, if not empty, goes
// on the first item; Telegram shows it under the album.
func sendMediaFileAlbum(bot *tgbotapi.BotAPI, message *tgbotapi.Message, group []downloader.MediaFile, captionHTML string) error {
	files := make([]interface{}, 0, len(group))
	for i, item := range group {
		caption, parseMode := "", ""
		if i == 0 && captionHTML != "" {
			caption, parseMode = captionHTML, tgbotapi.ModeHTML
		}

		if item.Type == "image" {
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(item.Path))
			photo.Caption = caption
			photo.ParseMode = parseMode
			files = append(files, photo)
			continue
		}

		video := tgbotapi.NewInputMediaVideo(tgbotapi.FilePath(item.Path))
		video.Caption = caption
		video.ParseMode = parseMode
		video.SupportsStreaming = true
		thumbnailPath, cleanupThumbnail, err := downloader.VideoThumbnail(item.Path)
		if err != nil {
			log.Printf("video thumbnail generation failed; sending without custom thumbnail: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
		} else {
			defer cleanupThumbnail()
			video.Thumb = tgbotapi.FilePath(thumbnailPath)
		}
		files = append(files, video)
	}

	album := tgbotapi.NewMediaGroup(message.Chat.ID, files)
	album.ReplyToMessageID = message.MessageID
	album.DisableNotification = true
	if _, err := bot.SendMediaGroup(album); err != nil {
		return fmt.Errorf("send album of %d items: %w", len(group), err)
	}
	return nil
}

// chunkSlice splits items into consecutive groups of at most size items,
// preserving order.
func chunkSlice[T any](items []T, size int) [][]T {
	var groups [][]T
	for start := 0; start < len(items); start += size {
		end := min(start+size, len(items))
		groups = append(groups, items[start:end])
	}
	return groups
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
	status.DisableNotification = true
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
		if err := sendYouTubeAudio(bot, message, youtubeAudioURL, track.Artist, track.Title, result); err == nil {
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

func sendYouTubeAudio(bot *tgbotapi.BotAPI, message *tgbotapi.Message, youtubeURL, artist, title, caption string) error {
	log.Printf("YouTube audio download started: chat_id=%d message_id=%d", message.Chat.ID, message.MessageID)
	filePath, cleanup, err := downloader.YouTubeAudio(youtubeURL)
	if err != nil {
		return fmt.Errorf("YouTube audio download failed: %w", err)
	}
	defer cleanup()
	log.Printf("YouTube audio download completed: path=%q chat_id=%d message_id=%d", filePath, message.Chat.ID, message.MessageID)

	audio := tgbotapi.NewAudio(message.Chat.ID, tgbotapi.FilePath(filePath))
	audio.ReplyToMessageID = message.MessageID
	audio.DisableNotification = true
	audio.Caption = caption
	audio.ParseMode = tgbotapi.ModeHTML
	audio.Title = strings.TrimSpace(title)
	audio.Performer = strings.TrimSpace(artist)

	videoID, err := extractYouTubeVideoID(youtubeURL)
	if err != nil {
		log.Printf("YouTube audio thumbnail skipped: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, err)
	} else if thumbnailPath, cleanupThumbnail, thumbnailErr := downloader.YouTubeAudioThumbnail(videoID); thumbnailErr != nil {
		log.Printf("YouTube audio thumbnail unavailable; sending without it: chat_id=%d message_id=%d: %v", message.Chat.ID, message.MessageID, thumbnailErr)
	} else {
		defer cleanupThumbnail()
		audio.Thumb = tgbotapi.FilePath(thumbnailPath)
	}

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
