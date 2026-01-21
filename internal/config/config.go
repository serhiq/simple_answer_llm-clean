package config

import (
	"fmt"
	"time"

	coreconfig "github.com/go-core-fx/config"
)

type Config struct {
	EvotorToken   string        `koanf:"evotor_token"`
	EvotorStoreID string        `koanf:"evotor_store_id"`
	LLMBaseURL    string        `koanf:"llm_base_url"`
	LLMAPIKey     string        `koanf:"llm_api_key"`
	LLMModel      string        `koanf:"llm_model"`
	Timeout       time.Duration `koanf:"timeout"`
	LogFile       string        `koanf:"log_file"`
	Debug         bool          `koanf:"debug"`
}

func New() (Config, error) {
	cfg := Config{
		Timeout: 20 * time.Second,
		LogFile: "./evotor-ai.log",
		Debug:   false,
	}

	if err := coreconfig.Load(&cfg); err != nil {
		return Config{}, fmt.Errorf("loading config: %w", err)
	}

	return cfg, nil
}
