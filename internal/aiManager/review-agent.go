package aiManager

import (
	"context"
	"fmt"
	"handshake-cli/internal/promptManager"
	"os"

	"github.com/sashabaranov/go-openai"
)

type AIProvider interface {
	AnalyzeCoverage(ctx context.Context, issueText string, gitDiff string) (string, error)
	ReviewCode(ctx context.Context, gitDiff string) (string, error)
}

type OpenRouterAdapter struct {
	model   string
	client  *openai.Client
	prompts *promptManager.Prompts
}

func NewOpenRouterAdapter(model string, prompts *promptManager.Prompts) (*OpenRouterAdapter, error) {
	if model == "" {
		return nil, fmt.Errorf("model must not be empty")
	}
	if prompts == nil {
		return nil, fmt.Errorf("promptManager must be provided")
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is not set")
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"

	return &OpenRouterAdapter{
		model:   model,
		client:  openai.NewClientWithConfig(config),
		prompts: prompts,
	}, nil
}

func (ora *OpenRouterAdapter) AnalyzeCoverage(ctx context.Context, issueText string, gitDiff string) (string, error) {
	userPrompt := fmt.Sprintf(ora.prompts.CoverageUser, issueText, gitDiff)
	return callOpenAICompatible(ctx, ora.client, ora.model, ora.prompts.CoverageSystem, userPrompt)
}

func (ora *OpenRouterAdapter) ReviewCode(ctx context.Context, gitDiff string) (string, error) {
	userPrompt := fmt.Sprintf(ora.prompts.ReviewUser, gitDiff)
	return callOpenAICompatible(ctx, ora.client, ora.model, ora.prompts.ReviewSystem, userPrompt)
}

type OllamaAdapter struct {
	model   string
	client  *openai.Client
	prompts *promptManager.Prompts
}

func NewOllamaAdapter(model string, host string, prompts *promptManager.Prompts) (*OllamaAdapter, error) {
	if model == "" {
		return nil, fmt.Errorf("model must not be empty")
	}
	if host == "" {
		host = "http://localhost:11434"
	}
	if prompts == nil {
		return nil, fmt.Errorf("promptManager must be provided")
	}

	config := openai.DefaultConfig("ollama-local")
	config.BaseURL = host + "/v1"

	return &OllamaAdapter{
		model:   model,
		client:  openai.NewClientWithConfig(config),
		prompts: prompts,
	}, nil
}

func (ola *OllamaAdapter) AnalyzeCoverage(ctx context.Context, issueText string, gitDiff string) (string, error) {
	userPrompt := fmt.Sprintf(ola.prompts.CoverageUser, issueText, gitDiff)
	return callOpenAICompatible(ctx, ola.client, ola.model, ola.prompts.CoverageSystem, userPrompt)
}

func (ola *OllamaAdapter) ReviewCode(ctx context.Context, gitDiff string) (string, error) {
	userPrompt := fmt.Sprintf(ola.prompts.ReviewUser, gitDiff)
	return callOpenAICompatible(ctx, ola.client, ola.model, ola.prompts.ReviewSystem, userPrompt)
}

func callOpenAICompatible(ctx context.Context, client *openai.Client, model string, systemPrompt string, userPrompt string) (string, error) {
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: userPrompt,
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from model")
	}

	return resp.Choices[0].Message.Content, nil
}
