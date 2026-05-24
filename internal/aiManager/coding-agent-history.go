package aiManager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CodingAgent interface {
	GetHistory() ([]string, error)
}

type GeminiSession struct {
	Messages []GeminiMessage `json:"messages"`
}

type GeminiMessage struct {
	Type    string      `json:"type"`
	Content interface{} `json:"content"`
}

type GeminiAdapter struct {
	ChatDirectory string
}

func parseMessageContent(msg GeminiMessage) string {
	var text string
	if msg.Type == "user" {
		if contentArr, ok := msg.Content.([]interface{}); ok && len(contentArr) > 0 {
			if contentMap, ok := contentArr[0].(map[string]interface{}); ok {
				if textVal, ok := contentMap["text"].(string); ok {
					text = textVal
				}
			}
		}
	} else if msg.Type == "gemini" {
		if strVal, ok := msg.Content.(string); ok {
			text = strVal
		}
	}
	return text
}

func (gemini *GeminiAdapter) GetHistory() ([]string, error) {
	home, _ := os.UserHomeDir()
	dirPath := filepath.Join(home, gemini.ChatDirectory)

	filesJSON, _ := filepath.Glob(filepath.Join(dirPath, "session-*.json"))
	filesJSONL, _ := filepath.Glob(filepath.Join(dirPath, "session-*.jsonl"))
	files := append(filesJSON, filesJSONL...)

	if len(files) == 0 {
		return nil, fmt.Errorf("could not find any session files in %s", dirPath)
	}

	var allHistory []string

	for _, file := range files {
		if strings.HasSuffix(file, ".jsonl") {
			f, err := os.Open(file)
			if err != nil {
				continue
			}
			scanner := bufio.NewScanner(f)
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 1024*1024)

			for scanner.Scan() {
				var msg GeminiMessage
				if err := json.Unmarshal(scanner.Bytes(), &msg); err == nil {
					text := parseMessageContent(msg)
					if text != "" {
						allHistory = append(allHistory, fmt.Sprintf("%s: %s", msg.Type, text))
					}
				}
			}
			f.Close()
		} else {
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}

			var session GeminiSession
			if err := json.Unmarshal(data, &session); err != nil {
				continue
			}

			for _, msg := range session.Messages {
				text := parseMessageContent(msg)
				if text != "" {
					allHistory = append(allHistory, fmt.Sprintf("%s: %s", msg.Type, text))
				}
			}
		}
	}

	return allHistory, nil
}
