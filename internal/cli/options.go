package cli

import "time"

type Options struct {
	Query         string
	EvotorToken   string
	EvotorStoreID string
	From          string
	To            string
	JSON          bool
	Debug         bool
	LogFile       string
	Timeout       time.Duration
	LLMBaseURL    string
	LLMAPIKey     string
	LLMModel      string
}
