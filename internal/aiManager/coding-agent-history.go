package aiManager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func (gemini *GeminiAdapter) GetHistory() ([]string, error) {
	home, _ := os.UserHomeDir()
	dirPath := filepath.Join(home, gemini.ChatDirectory)

	files, err := filepath.Glob(filepath.Join(dirPath, "session-*.json"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("could not find any session files in %s: %v", dirPath, err)
	}

	var allHistory []string

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var session GeminiSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		for _, msg := range session.Messages {
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

			if text != "" {
				formattedString := fmt.Sprintf("%s: %s", msg.Type, text)
				allHistory = append(allHistory, formattedString)
			}
		}
	}

	return allHistory, nil
}
