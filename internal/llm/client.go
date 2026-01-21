package llm

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"simple_answer_llm/internal/config"

	openrouter "github.com/revrost/go-openrouter"
	"go.uber.org/zap"
)

var ErrNotConfigured = errors.New("llm is not configured")

type Client struct {
	client  *openrouter.Client
	model   string
	logger  *zap.Logger
	enabled bool
}

func NewClient(cfg config.Config, logger *zap.Logger) (*Client, error) {
	logger = logger.Named("llm")
	model := strings.TrimSpace(cfg.LLMModel)
	apiKey := strings.TrimSpace(cfg.LLMAPIKey)

	if model == "" || apiKey == "" {
		logger.Warn("LLM config is incomplete; LLM calls will be disabled",
			zap.Bool("has_model", model != ""),
			zap.Bool("has_api_key", apiKey != ""),
		)
		return &Client{
			model:  model,
			logger: logger,
		}, nil
	}

	cfgClient := openrouter.DefaultConfig(apiKey)
	if strings.TrimSpace(cfg.LLMBaseURL) != "" {
		cfgClient.BaseURL = strings.TrimSpace(cfg.LLMBaseURL)
	}
	cfgClient.HTTPClient = &http.Client{Timeout: cfg.Timeout}

	return &Client{
		client:  openrouter.NewClientWithConfig(*cfgClient),
		model:   model,
		logger:  logger,
		enabled: true,
	}, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Client) Chat(ctx context.Context, systemPrompt, userPrompt string, tools []openrouter.Tool) (openrouter.ChatCompletionResponse, error) {
	if c == nil || !c.enabled || c.client == nil {
		return openrouter.ChatCompletionResponse{}, ErrNotConfigured
	}

	messages := []openrouter.ChatCompletionMessage{
		openrouter.SystemMessage(systemPrompt),
		openrouter.UserMessage(userPrompt),
	}
	return c.ChatWithMessages(ctx, messages, tools)
}

func (c *Client) ChatWithMessages(ctx context.Context, messages []openrouter.ChatCompletionMessage, tools []openrouter.Tool) (openrouter.ChatCompletionResponse, error) {
	if c == nil || !c.enabled || c.client == nil {
		return openrouter.ChatCompletionResponse{}, ErrNotConfigured
	}

	request := openrouter.ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
	}

	return c.client.CreateChatCompletion(ctx, request)
}
