package cli

import (
	"strings"

	openrouter "github.com/revrost/go-openrouter"
	"go.uber.org/zap"
)

const (
	defaultHistoryMaxMessages = 20
	defaultHistoryMaxTokens   = 2000
)

type SessionHistory struct {
	messages    []openrouter.ChatCompletionMessage
	maxMessages int
	maxTokens   int
	logger      *zap.Logger
}

func NewSessionHistory(maxMessages, maxTokens int, logger *zap.Logger) *SessionHistory {
	if maxMessages <= 0 {
		maxMessages = defaultHistoryMaxMessages
	}
	if maxTokens <= 0 {
		maxTokens = defaultHistoryMaxTokens
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SessionHistory{
		maxMessages: maxMessages,
		maxTokens:   maxTokens,
		logger:      logger,
	}
}

func (h *SessionHistory) Append(message openrouter.ChatCompletionMessage) {
	h.messages = append(h.messages, message)
	h.enforceLimits()
}

func (h *SessionHistory) GetMessages() []openrouter.ChatCompletionMessage {
	if len(h.messages) == 0 {
		return nil
	}
	out := make([]openrouter.ChatCompletionMessage, len(h.messages))
	copy(out, h.messages)
	return out
}

func (h *SessionHistory) Clear() {
	h.messages = nil
}

func (h *SessionHistory) TokenCount() int {
	return estimateTokens(h.messages)
}

func (h *SessionHistory) enforceLimits() {
	trimmed := false
	if h.maxMessages > 0 && len(h.messages) > h.maxMessages {
		h.messages = trimByCount(h.messages, h.maxMessages)
		trimmed = true
	}

	if h.maxTokens > 0 {
		for len(h.messages) > 1 && estimateTokens(h.messages) > h.maxTokens {
			h.messages = trimOldestNonSystem(h.messages)
			trimmed = true
		}
	}

	if trimmed {
		h.logger.Info("session history trimmed",
			zap.Int("messages", len(h.messages)),
			zap.Int("tokens", estimateTokens(h.messages)),
		)
	}
}

func trimByCount(messages []openrouter.ChatCompletionMessage, max int) []openrouter.ChatCompletionMessage {
	if len(messages) <= max {
		return messages
	}
	if len(messages) == 0 || max <= 0 {
		return nil
	}
	if messages[0].Role == openrouter.ChatMessageRoleSystem {
		keep := max - 1
		if keep <= 0 {
			return messages[:1]
		}
		start := len(messages) - keep
		if start < 1 {
			start = 1
		}
		trimmed := make([]openrouter.ChatCompletionMessage, 0, max)
		trimmed = append(trimmed, messages[0])
		trimmed = append(trimmed, messages[start:]...)
		return trimmed
	}
	return messages[len(messages)-max:]
}

func trimOldestNonSystem(messages []openrouter.ChatCompletionMessage) []openrouter.ChatCompletionMessage {
	if len(messages) == 0 {
		return nil
	}
	if messages[0].Role == openrouter.ChatMessageRoleSystem {
		if len(messages) <= 1 {
			return messages
		}
		return append(messages[:1], messages[2:]...)
	}
	return messages[1:]
}

func estimateTokens(messages []openrouter.ChatCompletionMessage) int {
	total := 0
	for _, msg := range messages {
		total += estimateTokensForMessage(msg)
	}
	return total
}

func estimateTokensForMessage(message openrouter.ChatCompletionMessage) int {
	total := 0
	if message.Content.Text != "" {
		total += len(strings.Fields(message.Content.Text))
	}
	if message.Content.Text == "" && len(message.Content.Multi) > 0 {
		for _, part := range message.Content.Multi {
			if part.Text != "" {
				total += len(strings.Fields(part.Text))
			}
		}
	}
	return total
}
