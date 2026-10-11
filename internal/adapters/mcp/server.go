package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"pkb/internal/usecase/domain"
)

const defaultSearchKnowledgeLimit = 10

type KnowledgeSearcher interface {
	SearchKnowledge(ctx context.Context, query string, limit int) ([]domain.KnowledgeItem, error)
}

type Server struct {
	server    *sdk.Server
	knowledge KnowledgeSearcher
}

func NewServer(knowledge KnowledgeSearcher) *Server {
	s := &Server{
		server: sdk.NewServer(
			&sdk.Implementation{
				Name:        "pkb",
				Title:       "Personal Knowledge Base",
				Description: "Access to the user's local personal knowledge base.",
				Version:     "0.1.0",
			},
			&sdk.ServerOptions{
				Instructions: "Use PKB when you need some extra information about user or his enviroment.",
				Capabilities: &sdk.ServerCapabilities{},
			},
		),
		knowledge: knowledge,
	}
	s.server.AddReceivingMiddleware(loggingMiddleware)
	sdk.AddTool(s.server, &sdk.Tool{
		Name:        "search_knowledge",
		Description: "Search the user's personal knowledge base by a natural-language query and return up to 10 complete knowledge items.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true},
	}, s.handleSearchKnowledge)
	return s
}

func (s *Server) Handler() http.Handler {
	return sdk.NewStreamableHTTPHandler(func(_ *http.Request) *sdk.Server {
		return s.server
	}, &sdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		Logger:       slog.Default(),
	})
}

type searchKnowledgeRequest struct {
	Query string `json:"query" jsonschema:"Natural-language query describing the information to find"`
}

func (s *Server) handleSearchKnowledge(
	ctx context.Context,
	_ *sdk.CallToolRequest,
	req searchKnowledgeRequest,
) (*sdk.CallToolResult, any, error) {

	if strings.TrimSpace(req.Query) == "" {
		return nil, nil, fmt.Errorf("query must be non empty")
	}

	items, err := s.knowledge.SearchKnowledge(ctx, req.Query, defaultSearchKnowledgeLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("search knowledge: %w", err)
	}
	if items == nil {
		items = []domain.KnowledgeItem{}
	}
	return nil, items, nil
}
