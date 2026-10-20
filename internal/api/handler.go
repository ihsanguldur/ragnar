package api

import (
	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/llm"
	"github.com/ihsanguldur/ragnar/internal/retrieval"
	"github.com/ihsanguldur/ragnar/internal/store"
)

type Handler struct {
	llm       llm.Provider
	retriever *retrieval.Retriever
	store     *store.Store
	chat      config.ChatConfig
	ingest    config.IngestConfig
	search    config.SearchConfig
}

func NewHandler(provider llm.Provider, retriever *retrieval.Retriever, st *store.Store, chat config.ChatConfig, ingest config.IngestConfig, search config.SearchConfig) *Handler {
	return &Handler{llm: provider, retriever: retriever, store: st, chat: chat, ingest: ingest, search: search}
}
