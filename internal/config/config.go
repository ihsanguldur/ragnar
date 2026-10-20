package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP     HTTPConfig     `mapstructure:"http"`
	Ollama   OllamaConfig   `mapstructure:"ollama"`
	Database DatabaseConfig `mapstructure:"database"`
	Ingest   IngestConfig   `mapstructure:"ingest"`
	Chat     ChatConfig     `mapstructure:"chat"`
	Search   SearchConfig   `mapstructure:"search"`
}

type HTTPConfig struct {
	Addr string `mapstructure:"addr"`
}

type OllamaConfig struct {
	URL        string `mapstructure:"url"`
	ChatModel  string `mapstructure:"chat_model"`
	EmbedModel string `mapstructure:"embed_model"`
	NumCtx     int    `mapstructure:"num_ctx"`
}

type DatabaseConfig struct {
	URL string `mapstructure:"url"`
}

type IngestConfig struct {
	UploadDir      string        `mapstructure:"upload_dir"`
	MaxUploadMB    int64         `mapstructure:"max_upload_mb"`
	Workers        int           `mapstructure:"workers"`
	PollInterval   time.Duration `mapstructure:"poll_interval"`
	ExtractTimeout time.Duration `mapstructure:"extract_timeout"`
	ChunkSize      int           `mapstructure:"chunk_size"`
	ChunkOverlap   int           `mapstructure:"chunk_overlap"`
	EmbedBatchSize int           `mapstructure:"embed_batch_size"`
	EmbedTimeout   time.Duration `mapstructure:"embed_timeout"`
	MaxAttempts    int           `mapstructure:"max_attempts"`
	RetryBackoff   time.Duration `mapstructure:"retry_backoff"`
}

type ChatConfig struct {
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxTokens    int           `mapstructure:"max_tokens"`
	Temperature  float64       `mapstructure:"temperature"`
	SystemPrompt string        `mapstructure:"system_prompt"`
}

type SearchConfig struct {
	TopK    int           `mapstructure:"top_k"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetEnvPrefix("RAGNAR")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.HTTP.Addr == "" {
		return errors.New("config: http.addr is required")
	}
	if c.Ollama.URL == "" {
		return errors.New("config: ollama.url is required")
	}
	if c.Ollama.ChatModel == "" {
		return errors.New("config: ollama.chat_model is required")
	}
	if c.Ollama.EmbedModel == "" {
		return errors.New("config: ollama.embed_model is required")
	}
	if c.Ollama.NumCtx <= 0 {
		return errors.New("config: ollama.num_ctx must be positive")
	}
	if c.Database.URL == "" {
		return errors.New("config: database.url is required")
	}
	if c.Ingest.UploadDir == "" {
		return errors.New("config: ingest.upload_dir is required")
	}
	if c.Ingest.MaxUploadMB <= 0 {
		return errors.New("config: ingest.max_upload_mb must be positive")
	}
	if c.Ingest.Workers <= 0 {
		return errors.New("config: ingest.workers must be positive")
	}
	if c.Ingest.PollInterval <= 0 {
		return errors.New("config: ingest.poll_interval must be positive")
	}
	if c.Ingest.ExtractTimeout <= 0 {
		return errors.New("config: ingest.extract_timeout must be positive")
	}
	if c.Ingest.ChunkSize <= 0 {
		return errors.New("config: ingest.chunk_size must be positive")
	}
	if c.Ingest.ChunkOverlap < 0 || c.Ingest.ChunkOverlap >= c.Ingest.ChunkSize {
		return errors.New("config: ingest.chunk_overlap must be non-negative and smaller than ingest.chunk_size")
	}
	if c.Ingest.EmbedBatchSize <= 0 {
		return errors.New("config: ingest.embed_batch_size must be positive")
	}
	if c.Ingest.EmbedTimeout <= 0 {
		return errors.New("config: ingest.embed_timeout must be positive")
	}
	if c.Ingest.MaxAttempts <= 0 {
		return errors.New("config: ingest.max_attempts must be positive")
	}
	if c.Ingest.RetryBackoff <= 0 {
		return errors.New("config: ingest.retry_backoff must be positive")
	}
	if c.Chat.Timeout <= 0 {
		return errors.New("config: chat.timeout must be positive")
	}
	if c.Chat.MaxTokens <= 0 {
		return errors.New("config: chat.max_tokens must be positive")
	}
	if c.Chat.SystemPrompt == "" {
		return errors.New("config: chat.system_prompt is required")
	}
	if c.Search.TopK <= 0 {
		return errors.New("config: search.top_k must be positive")
	}
	if c.Search.Timeout <= 0 {
		return errors.New("config: search.timeout must be positive")
	}
	return nil
}
