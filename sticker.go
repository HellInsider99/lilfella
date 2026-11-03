package main

import (
	"bytes"
	"image/png"
	"io"
	"log"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/image/webp"
)

func handleSticker(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	sticker := message.Sticker
	file, err := bot.GetFile(tgbotapi.FileConfig{FileID: sticker.FileID})
	if err != nil {
		log.Printf("failed to get sticker file %s: %v", sticker.FileID, err)
		return
	}

	extension := ".webp"
	if sticker.IsAnimated {
		extension = ".tgs"
	} else if strings.HasSuffix(strings.ToLower(file.FilePath), ".webm") {
		extension = ".webm"
	}
	fileURL := "https://api.telegram.org/file/bot" + bot.Token + "/" + file.FilePath
	response, err := http.Get(fileURL)
	if err != nil {
		log.Printf("failed to download sticker file %s: %v", file.FilePath, err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		log.Printf("failed to download sticker file %s: %s", file.FilePath, response.Status)
		return
	}
	fileData, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("failed to read sticker file %s: %v", file.FilePath, err)
		return
	}

	fileName := "sticker_" + sticker.FileUniqueID + extension
	if strings.HasSuffix(strings.ToLower(file.FilePath), ".webp") {
		imageData, err := webp.Decode(bytes.NewReader(fileData))
		if err != nil {
			log.Printf("failed to decode WebP sticker file %s: %v", file.FilePath, err)
			return
		}

		var pngData bytes.Buffer
		if err := png.Encode(&pngData, imageData); err != nil {
			log.Printf("failed to encode PNG sticker file %s: %v", file.FilePath, err)
			return
		}
		fileData = pngData.Bytes()
		fileName = "sticker_" + sticker.FileUniqueID + ".png"
	}

	document := tgbotapi.NewDocument(message.Chat.ID, tgbotapi.FileBytes{
		Name:  fileName,
		Bytes: fileData,
	})
	document.ReplyToMessageID = message.MessageID
	if _, err := bot.Send(document); err != nil {
		log.Printf("failed to send sticker document %s: %v", file.FilePath, err)
	}
}
