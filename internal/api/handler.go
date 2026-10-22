package api

import (
	"github.com/ihsanguldur/ragnar/internal/config"
	"github.com/ihsanguldur/ragnar/internal/rag"
	"github.com/ihsanguldur/ragnar/internal/retrieval"
	"github.com/ihsanguldur/ragnar/internal/store"
)

type Handler struct {
	retriever *retrieval.Retriever
	rag       *rag.Service
	store     *store.Store
	chat      config.ChatConfig
	ingest    config.IngestConfig
	search    config.SearchConfig
}

func NewHandler(retriever *retrieval.Retriever, ragService *rag.Service, st *store.Store, chat config.ChatConfig, ingest config.IngestConfig, search config.SearchConfig) *Handler {
	return &Handler{retriever: retriever, rag: ragService, store: st, chat: chat, ingest: ingest, search: search}
}
