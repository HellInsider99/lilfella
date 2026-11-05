package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

const defaultLogFile = "logs/bot.log"

func initLogging() (io.Closer, error) {
	logPath := os.Getenv("BOT_LOG_FILE")
	if logPath == "" {
		logPath = defaultLogFile
	}

	if dir := filepath.Dir(logPath); dir != "." {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", logPath, err)
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(io.MultiWriter(os.Stdout, file))
	log.Printf("logging initialized: file=%q", logPath)
	return file, nil
}
