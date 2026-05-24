package aiManager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sashabaranov/go-openai"
)

const syncCacheFile = ".handshake_sync.json"

type VectorizeResult struct {
	Embedded int
	Skipped  int
	Errors   int
	Project  string
	Total    int
}

func generateHash(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func loadSyncCache() map[string]bool {
	cache := make(map[string]bool)
	data, err := os.ReadFile(syncCacheFile)
	if err == nil {
		json.Unmarshal(data, &cache)
	}
	return cache
}

func saveSyncCache(cache map[string]bool) {
	data, _ := json.MarshalIndent(cache, "", "  ")
	os.WriteFile(syncCacheFile, data, 0644)
}

func ClearSyncCache() error {
	err := os.Remove(syncCacheFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type MatchPayload struct {
	QueryEmbedding []float32 `json:"query_embedding"`
	MatchThreshold float64   `json:"match_threshold"`
	MatchCount     int       `json:"match_count"`
	ProjectName    string    `json:"p_project_name"`
}

type MatchResult struct {
	Content string `json:"content"`
}

func SearchSimilarHistory(vector []float32, projectName string, limit int) (string, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		return "", fmt.Errorf("missing SUPABASE_URL or SUPABASE_KEY environment variables")
	}

	url := fmt.Sprintf("%s/rest/v1/rpc/match_ai_memory", supabaseURL)

	payload := MatchPayload{
		QueryEmbedding: vector,
		MatchThreshold: 0.5,
		MatchCount:     limit,
		ProjectName:    projectName,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("apikey", supabaseKey)
	req.Header.Set("Authorization", "Bearer "+supabaseKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("supabase rejected the request with status %d", resp.StatusCode)
	}

	var results []MatchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", err
	}

	var sb strings.Builder
	for i, res := range results {
		sb.WriteString(fmt.Sprintf("--- Historical Interaction %d ---\n%s\n\n", i+1, res.Content))
	}

	return sb.String(), nil
}

func GetEmbedding(text string) ([]float32, error) {
	config := openai.DefaultConfig("ollama-local")
	config.BaseURL = "http://localhost:11434/v1"
	client := openai.NewClientWithConfig(config)

	req := openai.EmbeddingRequest{
		Input: []string{text},
		Model: "nomic-embed-text",
	}

	resp, err := client.CreateEmbeddings(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return resp.Data[0].Embedding, nil
}

type SupabasePayload struct {
	ProjectName string    `json:"project_name"`
	Content     string    `json:"content"`
	Hash        string    `json:"hash"`
	Embedding   []float32 `json:"embedding"`
}

func sendToSupabase(chatText string, vector []float32, hash string, projectName string) error {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		return fmt.Errorf("missing SUPABASE_URL or SUPABASE_KEY environment variables")
	}

	url := fmt.Sprintf("%s/rest/v1/ai_memory", supabaseURL)

	payload := SupabasePayload{
		ProjectName: projectName,
		Content:     chatText,
		Hash:        hash,
		Embedding:   vector,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("apikey", supabaseKey)
	req.Header.Set("Authorization", "Bearer "+supabaseKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("supabase rejected the request with status %d", resp.StatusCode)
	}

	return nil
}

func RunVectorizeJob() (VectorizeResult, error) {
	var result VectorizeResult

	cwd, err := os.Getwd()
	if err != nil {
		return result, fmt.Errorf("could not get working directory: %v", err)
	}

	result.Project = filepath.Base(cwd)
	dynamicPath := filepath.Join(".gemini/tmp/", result.Project, "/chats")

	var agent CodingAgent
	agent = &GeminiAdapter{ChatDirectory: dynamicPath}

	historyData, err := agent.GetHistory()
	if err != nil {
		return result, err
	}

	result.Total = len(historyData)

	cache := loadSyncCache()

	for _, chatText := range historyData {
		hash := generateHash(chatText)

		if cache[hash] {
			result.Skipped++
			continue
		}

		vector, err := GetEmbedding(chatText)
		if err != nil {
			result.Errors++
			continue
		}

		if err := sendToSupabase(chatText, vector, hash, result.Project); err != nil {
			result.Errors++
			continue
		}

		cache[hash] = true
		result.Embedded++
	}

	saveSyncCache(cache)
	return result, nil
}
