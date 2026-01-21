package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"simple_answer_llm/internal/config"
	"simple_answer_llm/internal/evotor"
	"simple_answer_llm/internal/llm"

	openrouter "github.com/revrost/go-openrouter"
	"go.uber.org/zap"
)

type Runner struct {
	options   Options
	logger    *zap.Logger
	llmClient *llm.Client
}

func NewRunner(cfg config.Config, logger *zap.Logger, llmClient *llm.Client) *Runner {
	logger = logger.Named("cli")
	opts := Options{
		EvotorToken:   cfg.EvotorToken,
		EvotorStoreID: cfg.EvotorStoreID,
		LLMBaseURL:    cfg.LLMBaseURL,
		LLMAPIKey:     cfg.LLMAPIKey,
		LLMModel:      cfg.LLMModel,
		Timeout:       cfg.Timeout,
		LogFile:       cfg.LogFile,
		Debug:         cfg.Debug,
	}

	return &Runner{
		options:   opts,
		logger:    logger,
		llmClient: llmClient,
	}
}

func (r *Runner) Execute() error {
	return runCLI(&r.options, r.logger)
}

func runCLI(opts *Options, logger *zap.Logger) error {
	var timeoutSeconds int

	fs := flag.NewFlagSet("evotor-ai", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [query]\n", fs.Name())
		fs.PrintDefaults()
	}

	fs.StringVar(&opts.EvotorToken, "token", opts.EvotorToken, "Evotor API token (EVOTOR_TOKEN)")
	fs.StringVar(&opts.EvotorStoreID, "store-id", opts.EvotorStoreID, "Evotor store ID (EVOTOR_STORE_ID)")
	fs.StringVar(&opts.From, "from", "", "Start date (YYYY-MM-DD)")
	fs.StringVar(&opts.To, "to", "", "End date (YYYY-MM-DD)")
	fs.BoolVar(&opts.JSON, "json", false, "Output JSON format")
	fs.BoolVar(&opts.Debug, "debug", opts.Debug, "Enable debug logging")
	fs.StringVar(&opts.LogFile, "log-file", opts.LogFile, "Log file path")
	fs.IntVar(&timeoutSeconds, "timeout", int(opts.Timeout.Seconds()), "Timeout in seconds")
	fs.StringVar(&opts.LLMBaseURL, "llm-base-url", opts.LLMBaseURL, "LLM base URL (LLM_BASE_URL)")
	fs.StringVar(&opts.LLMAPIKey, "llm-api-key", opts.LLMAPIKey, "LLM API key (LLM_API_KEY)")
	fs.StringVar(&opts.LLMModel, "llm-model", opts.LLMModel, "LLM model (LLM_MODEL)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			fs.Usage()
			return nil
		}
		return err
	}

	if timeoutSeconds > 0 {
		opts.Timeout = time.Duration(timeoutSeconds) * time.Second
	}

	args := fs.Args()
	if len(args) > 1 {
		return fmt.Errorf("only one query argument is supported")
	}
	if len(args) == 1 {
		opts.Query = strings.TrimSpace(args[0])
	}

	updatedLLMClient, err := newLLMClientFromOptions(opts, logger)
	if err != nil {
		return err
	}
	evotorClient := newEvotorClientFromOptions(opts, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	if opts.Query == "" {
		return runREPL(ctx, opts, logger, updatedLLMClient, evotorClient)
	}
	return runOneShot(ctx, opts, logger, updatedLLMClient, evotorClient, opts.Query)
}

func newLLMClientFromOptions(opts *Options, logger *zap.Logger) (*llm.Client, error) {
	cfg := config.Config{
		LLMBaseURL: opts.LLMBaseURL,
		LLMAPIKey:  opts.LLMAPIKey,
		LLMModel:   opts.LLMModel,
		Timeout:    opts.Timeout,
	}
	return llm.NewClient(cfg, logger)
}

func newEvotorClientFromOptions(opts *Options, logger *zap.Logger) *evotor.Client {
	cfg := config.Config{
		EvotorToken:   opts.EvotorToken,
		EvotorStoreID: opts.EvotorStoreID,
		Timeout:       opts.Timeout,
	}
	return evotor.NewClient(cfg, logger)
}

func runOneShot(ctx context.Context, opts *Options, logger *zap.Logger, llmClient *llm.Client, evotorClient *evotor.Client, query string) error {
	return handleQuery(ctx, opts, logger, llmClient, evotorClient, query, false, nil)
}

func runREPL(ctx context.Context, opts *Options, logger *zap.Logger, llmClient *llm.Client, evotorClient *evotor.Client) error {
	reader := bufio.NewScanner(os.Stdin)
	history := NewSessionHistory(defaultHistoryMaxMessages, defaultHistoryMaxTokens, logger)
	history.Append(openrouter.SystemMessage(llm.SystemPromptWithContext(true)))
	fmt.Fprintln(os.Stdout, "Evotor AI CLI (type 'exit' to quit)")

	for {
		fmt.Fprint(os.Stdout, "> ")
		if !reader.Scan() {
			return reader.Err()
		}

		line := strings.TrimSpace(reader.Text())
		switch strings.ToLower(line) {
		case "":
			continue
		case "/clear":
			history.Clear()
			history.Append(openrouter.SystemMessage(llm.SystemPromptWithContext(true)))
			fmt.Fprintln(os.Stdout, "История очищена.")
			continue
		case "/history":
			printHistory(history)
			continue
		case "exit", "quit":
			return nil
		}

		if err := handleQuery(ctx, opts, logger, llmClient, evotorClient, line, true, history); err != nil {
			return err
		}
	}
}

func printHistory(history *SessionHistory) {
	if history == nil {
		fmt.Fprintln(os.Stdout, "История недоступна.")
		return
	}
	messages := history.GetMessages()
	if len(messages) == 0 {
		fmt.Fprintln(os.Stdout, "История пуста.")
		return
	}
	fmt.Fprintf(os.Stdout, "История (%d сообщений, ~%d токенов):\n", len(messages), history.TokenCount())
	for i, msg := range messages {
		preview := messagePreview(msg)
		if preview == "" {
			preview = "(empty)"
		}
		fmt.Fprintf(os.Stdout, "%d) %s: %s\n", i+1, msg.Role, preview)
	}
}

func messagePreview(msg openrouter.ChatCompletionMessage) string {
	text := strings.TrimSpace(msg.Content.Text)
	if text == "" && len(msg.Content.Multi) > 0 {
		for _, part := range msg.Content.Multi {
			if strings.TrimSpace(part.Text) != "" {
				text = strings.TrimSpace(part.Text)
				break
			}
		}
	}
	if text == "" {
		return ""
	}
	const maxLen = 120
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func handleQuery(ctx context.Context, opts *Options, logger *zap.Logger, llmClient *llm.Client, evotorClient *evotor.Client, query string, interactive bool, history *SessionHistory) error {
	logger.Info("query received",
		zap.String("query", query),
		zap.String("store_id", opts.EvotorStoreID),
		zap.String("from", opts.From),
		zap.String("to", opts.To),
		zap.Bool("json", opts.JSON),
	)

	if strings.TrimSpace(opts.EvotorToken) == "" {
		return evotor.ErrMissingToken
	}

	response, err := runLLMAgent(ctx, opts, logger, llmClient, evotorClient, query, interactive, history)
	if err != nil {
		return err
	}
	logResponse(logger, response)
	return writeResponse(opts, response)
}

type jsonResponse struct {
	Query          string           `json:"query"`
	AppliedFilters appliedFilters   `json:"applied_filters,omitempty"`
	AnswerText     string           `json:"answer_text"`
	Results        any              `json:"results,omitempty"`
	ToolCalls      []toolCallRecord `json:"tool_calls,omitempty"`
}

func writeResponse(opts *Options, resp response) error {
	if opts.JSON {
		return writeJSONResponse(resp)
	}
	return writeHumanResponse(resp)
}

func writeJSONResponse(resp response) error {
	payload := jsonResponse{
		Query:          resp.Query,
		AppliedFilters: resp.AppliedFilters,
		AnswerText:     strings.TrimSpace(resp.AnswerText),
		Results:        resp.Results,
		ToolCalls:      resp.ToolCalls,
	}

	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(payload)
}

func writeHumanResponse(resp response) error {
	answer := strings.TrimSpace(resp.AnswerText)

	fmt.Fprintln(os.Stdout, "Ответ:")
	if answer != "" {
		fmt.Fprintf(os.Stdout, "- %s\n", answer)
	} else {
		fmt.Fprintln(os.Stdout, "- (empty response)")
	}

	if hasFilters(resp.AppliedFilters) {
		fmt.Fprintln(os.Stdout, "\nФильтры:")
		writeFilters(resp.AppliedFilters)
	}

	if resp.Results != nil {
		fmt.Fprintln(os.Stdout, "\nРезультаты:")
		writeResults(resp.Results)
	}

	if strings.TrimSpace(resp.NextStep) != "" {
		fmt.Fprintln(os.Stdout, "\nСледующий шаг:")
		fmt.Fprintf(os.Stdout, "- %s\n", strings.TrimSpace(resp.NextStep))
	}

	fmt.Fprintf(os.Stdout, "\nЗапрос: %s\n", resp.Query)
	return nil
}
