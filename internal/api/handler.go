package api

import (
	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/llm"
	"github.com/ihsanguldur/ragnar/internal/store"
)

type Handler struct {
	llm    llm.Provider
	store  *store.Store
	chat   config.ChatConfig
	ingest config.IngestConfig
}

func NewHandler(provider llm.Provider, st *store.Store, chat config.ChatConfig, ingest config.IngestConfig) *Handler {
	return &Handler{llm: provider, store: st, chat: chat, ingest: ingest}
}
