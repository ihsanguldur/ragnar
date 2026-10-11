package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTP   HTTPConfig   `mapstructure:"http"`
	Ollama OllamaConfig `mapstructure:"ollama"`
	Chat   ChatConfig   `mapstructure:"chat"`
}

type HTTPConfig struct {
	Addr string `mapstructure:"addr"`
}

type OllamaConfig struct {
	URL        string `mapstructure:"url"`
	ChatModel  string `mapstructure:"chat_model"`
	EmbedModel string `mapstructure:"embed_model"`
}

type ChatConfig struct {
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxTokens    int           `mapstructure:"max_tokens"`
	Temperature  float64       `mapstructure:"temperature"`
	SystemPrompt string        `mapstructure:"system_prompt"`
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
	if c.Chat.Timeout <= 0 {
		return errors.New("config: chat.timeout must be positive")
	}
	if c.Chat.MaxTokens <= 0 {
		return errors.New("config: chat.max_tokens must be positive")
	}
	if c.Chat.SystemPrompt == "" {
		return errors.New("config: chat.system_prompt is required")
	}
	return nil
}
