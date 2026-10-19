package api

import (
	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/embed"
	"github.com/ihsanguldur/ragnar/internal/llm"
	"github.com/ihsanguldur/ragnar/internal/store"
)

type Handler struct {
	llm      llm.Provider
	embedder embed.Embedder
	store    *store.Store
	chat     config.ChatConfig
	ingest   config.IngestConfig
	search   config.SearchConfig
}

func NewHandler(provider llm.Provider, embedder embed.Embedder, st *store.Store, chat config.ChatConfig, ingest config.IngestConfig, search config.SearchConfig) *Handler {
	return &Handler{llm: provider, embedder: embedder, store: st, chat: chat, ingest: ingest, search: search}
}
