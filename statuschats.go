package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// defaultStatusChatListFile is used when UN_VANYA_LIST_FILE is not set.
const defaultStatusChatListFile = "un-vanya-list.json"

// statusChatList ("un-vanya-list") holds the chats where the bot posts a
// "downloading" status message while fetching media. In every other chat it
// stays quiet and only sends the media itself.
type statusChatList map[int64]bool

func (list statusChatList) Contains(chatID int64) bool {
	return list[chatID]
}

// loadStatusChatList reads the list from UN_VANYA_LIST_FILE, or
// un-vanya-list.json in the working directory. The file is a JSON array of
// chat IDs, for example [123456789, -1001234567890]. A missing file gives an
// empty list, so no chat gets status messages.
func loadStatusChatList() (statusChatList, string, error) {
	path := strings.TrimSpace(os.Getenv("UN_VANYA_LIST_FILE"))
	if path == "" {
		path = defaultStatusChatListFile
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return statusChatList{}, path, nil
	}
	if err != nil {
		return statusChatList{}, path, fmt.Errorf("read %s: %w", path, err)
	}
	list, err := parseStatusChatList(data)
	if err != nil {
		return statusChatList{}, path, fmt.Errorf("parse %s: %w", path, err)
	}
	return list, path, nil
}

func parseStatusChatList(data []byte) (statusChatList, error) {
	var ids []int64
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("want a JSON array of chat IDs: %w", err)
	}
	list := make(statusChatList, len(ids))
	for _, id := range ids {
		list[id] = true
	}
	return list, nil
}
