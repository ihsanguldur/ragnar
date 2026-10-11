package api

import (
	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/llm"
)

type Handler struct {
	llm  llm.Provider
	chat config.ChatConfig
}

func NewHandler(provider llm.Provider, chat config.ChatConfig) *Handler {
	return &Handler{llm: provider, chat: chat}
}
