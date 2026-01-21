package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"simple_answer_llm/internal/evotor"
	"simple_answer_llm/internal/llm"

	openrouter "github.com/revrost/go-openrouter"
	"go.uber.org/zap"
)

const maxToolRounds = 4

func runLLMAgent(ctx context.Context, opts *Options, logger *zap.Logger, llmClient *llm.Client, evotorClient *evotor.Client, query string, interactive bool, history *SessionHistory) (response, error) {
	if llmClient == nil || !llmClient.Enabled() {
		return response{}, llm.ErrNotConfigured
	}

	var messages []openrouter.ChatCompletionMessage
	if history != nil {
		if len(history.GetMessages()) == 0 {
			history.Append(openrouter.SystemMessage(llm.SystemPromptWithContext(interactive)))
		}
		history.Append(openrouter.UserMessage(query))
		messages = history.GetMessages()
	} else {
		messages = []openrouter.ChatCompletionMessage{
			openrouter.SystemMessage(llm.SystemPromptWithContext(interactive)),
			openrouter.UserMessage(query),
		}
	}

	var toolCalls []toolCallRecord

	for round := 0; round < maxToolRounds; round++ {
		if history != nil {
			messages = history.GetMessages()
		}
		resp, err := llmClient.ChatWithMessages(ctx, messages, llm.ToolSchemas())
		if err != nil {
			return response{}, err
		}
		logLLMUsage(logger, resp)
		if len(resp.Choices) == 0 {
			return response{}, fmt.Errorf("llm returned empty response")
		}

		msg := resp.Choices[0].Message
		logger.Debug("llm response",
			zap.String("content", msg.Content.Text),
			zap.Int("tool_calls", len(msg.ToolCalls)),
		)

		if len(msg.ToolCalls) == 0 {
			if history != nil {
				history.Append(msg)
			}
			return response{
				Query:      query,
				AnswerText: strings.TrimSpace(msg.Content.Text),
				ToolCalls:  toolCalls,
			}, nil
		}

		if history != nil {
			history.Append(msg)
		} else {
			messages = append(messages, msg)
		}
		toolMsgs, callRecords, err := executeToolCalls(ctx, logger, evotorClient, opts, msg.ToolCalls)
		toolCalls = append(toolCalls, callRecords...)
		if history != nil && len(toolMsgs) > 0 {
			for _, toolMsg := range toolMsgs {
				history.Append(toolMsg)
			}
		} else if len(toolMsgs) > 0 {
			messages = append(messages, toolMsgs...)
		}
		if err != nil {
			return response{
				Query:      query,
				AnswerText: friendlyEvotorError(err),
				ToolCalls:  toolCalls,
			}, nil
		}
	}

	return response{
		Query:      query,
		AnswerText: "Не удалось завершить запрос: превышен лимит шагов.",
		ToolCalls:  toolCalls,
		NextStep:   "Уточните запрос или сузьте период/магазин.",
	}, nil
}

func executeToolCalls(ctx context.Context, logger *zap.Logger, evotorClient *evotor.Client, opts *Options, calls []llm.ToolCall) ([]openrouter.ChatCompletionMessage, []toolCallRecord, error) {
	if evotorClient == nil {
		return nil, nil, fmt.Errorf("evotor client is not configured")
	}

	toolMessages := make([]openrouter.ChatCompletionMessage, 0, len(calls))
	records := make([]toolCallRecord, 0, len(calls))

	for _, call := range calls {
		args := map[string]any{}
		if call.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				record := toolCallRecord{
					Name: call.Function.Name,
					Args: args,
					MS:   0,
					OK:   false,
					Err:  fmt.Sprintf("invalid tool args: %v", err),
				}
				records = append(records, record)
				logToolRecord(logger, record)
				toolMessages = append(toolMessages, openrouter.ToolMessage(call.ID, toolErrorPayload(record.Err)))
				continue
			}
		}

		result, record, err := dispatchToolCall(ctx, logger, evotorClient, opts, call.Function.Name, args)
		if err != nil {
			records = append(records, record)
			toolMessages = append(toolMessages, openrouter.ToolMessage(call.ID, toolErrorPayload(err.Error())))
			return toolMessages, records, err
		}

		payload, err := json.Marshal(result)
		if err != nil {
			record.OK = false
			record.Err = err.Error()
			records = append(records, record)
			toolMessages = append(toolMessages, openrouter.ToolMessage(call.ID, toolErrorPayload(err.Error())))
			return toolMessages, records, err
		}
		records = append(records, record)
		toolMessages = append(toolMessages, openrouter.ToolMessage(call.ID, string(payload)))
	}

	return toolMessages, records, nil
}

func dispatchToolCall(ctx context.Context, logger *zap.Logger, evotorClient *evotor.Client, opts *Options, name string, args map[string]any) (any, toolCallRecord, error) {
	switch name {
	case "GetSalesMetrics":
		from, err := getTimeArg(args, "from")
		if err != nil {
			return nil, toolCallRecord{Name: name, Args: args, OK: false, Err: err.Error()}, err
		}
		to, err := getTimeArg(args, "to")
		if err != nil {
			return nil, toolCallRecord{Name: name, Args: args, OK: false, Err: err.Error()}, err
		}
		storeID := getStoreIDArg(args, opts.EvotorStoreID)
		documentType, _ := getStringArg(args, "document_type")
		return trackCall(logger, name, args, func() (evotor.SalesMetrics, error) {
			var docType *string
			if documentType != "" {
				docType = &documentType
			}
			return evotorClient.GetSalesMetrics(ctx, from, to, optionalString(storeID), docType)
		})
	case "ListStores":
		return trackCall(logger, name, args, func() ([]evotor.Store, error) {
			return evotorClient.ListStores(ctx)
		})
	case "SearchItems":
		query, _ := getStringArg(args, "query")
		limit := getIntArg(args, "limit", defaultOutputLimit)
		storeID := getStoreIDArg(args, opts.EvotorStoreID)
		return trackCall(logger, name, args, func() ([]evotor.Item, error) {
			return evotorClient.SearchItems(ctx, query, limit, optionalString(storeID))
		})
	case "SearchDocuments":
		from, err := getTimeArg(args, "from")
		if err != nil {
			return nil, toolCallRecord{Name: name, Args: args, OK: false, Err: err.Error()}, err
		}
		to, err := getTimeArg(args, "to")
		if err != nil {
			return nil, toolCallRecord{Name: name, Args: args, OK: false, Err: err.Error()}, err
		}
		limit := getIntArg(args, "limit", defaultDocLimit)
		offset := getIntArg(args, "offset", 0)
		storeID := getStoreIDArg(args, opts.EvotorStoreID)
		itemQuery, _ := getStringArg(args, "item_query")
		if strings.TrimSpace(itemQuery) == "" {
			return trackCall(logger, name, args, func() ([]evotor.DocumentShort, error) {
				return evotorClient.SearchDocuments(ctx, from, to, optionalString(storeID), limit, offset)
			})
		}

		documents, record, err := trackCall(logger, name, args, func() ([]evotor.DocumentShort, error) {
			return evotorClient.SearchDocuments(ctx, from, to, optionalString(storeID), limit, offset)
		})
		if err != nil {
			return nil, record, err
		}
		filtered, filterErr := filterDocumentsByItem(ctx, logger, evotorClient, storeID, documents, itemQuery)
		return filtered, record, filterErr
	case "GetDocument":
		docID, _ := getStringArg(args, "doc_id")
		storeID := getStoreIDArg(args, opts.EvotorStoreID)
		return trackCall(logger, name, args, func() (evotor.DocumentFull, error) {
			return evotorClient.GetDocument(ctx, docID, optionalString(storeID))
		})
	default:
		err := fmt.Errorf("unknown tool: %s", name)
		return nil, toolCallRecord{Name: name, Args: args, OK: false, Err: err.Error()}, err
	}
}

func getStringArg(args map[string]any, key string) (string, bool) {
	value, ok := args[key]
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func getIntArg(args map[string]any, key string, fallback int) int {
	value, ok := args[key]
	if !ok {
		return fallback
	}
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func getTimeArg(args map[string]any, key string) (time.Time, error) {
	value, ok := getStringArg(args, key)
	if !ok || value == "" {
		return time.Time{}, fmt.Errorf("missing %s", key)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}

func getStoreIDArg(args map[string]any, fallback string) string {
	if value, ok := getStringArg(args, "store_id"); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func toolErrorPayload(message string) string {
	payload := map[string]string{
		"error": message,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, message)
	}
	return string(encoded)
}

func filterDocumentsByItem(ctx context.Context, logger *zap.Logger, evotorClient *evotor.Client, storeID string, documents []evotor.DocumentShort, itemQuery string) ([]evotor.DocumentShort, error) {
	needle := strings.ToLower(strings.TrimSpace(itemQuery))
	if needle == "" {
		return documents, nil
	}

	matches := make([]evotor.DocumentShort, 0, defaultOutputLimit)
	for _, doc := range documents {
		fullDoc, _, err := trackCall(logger, "GetDocument", map[string]any{"doc_id": doc.ID}, func() (evotor.DocumentFull, error) {
			return evotorClient.GetDocument(ctx, doc.ID, optionalString(storeID))
		})
		if err != nil {
			return nil, err
		}
		if documentHasItem(fullDoc, needle) {
			matches = append(matches, evotor.DocumentShort{
				ID:        fullDoc.ID,
				Type:      fullDoc.Type,
				CloseDate: fullDoc.CloseDate,
				DeviceID:  fullDoc.DeviceID,
				StoreID:   fullDoc.StoreID,
				Body:      fullDoc.Body,
				Total:     fullDoc.Total,
			})
			if len(matches) >= defaultOutputLimit {
				break
			}
		}
	}
	return matches, nil
}

func logToolRecord(logger *zap.Logger, record toolCallRecord) {
	if logger == nil {
		return
	}
	logger.Info("tool call",
		zap.String("name", record.Name),
		zap.Any("args", record.Args),
		zap.Int64("ms", record.MS),
		zap.Bool("ok", record.OK),
		zap.String("err", record.Err),
	)
}

func logLLMUsage(logger *zap.Logger, resp openrouter.ChatCompletionResponse) {
	if resp.Usage == nil {
		return
	}
	logger.Info("llm usage",
		zap.Int("prompt_tokens", resp.Usage.PromptTokens),
		zap.Int("completion_tokens", resp.Usage.CompletionTokens),
		zap.Int("total_tokens", resp.Usage.TotalTokens),
		zap.Float64("cost", resp.Usage.Cost),
	)
}
