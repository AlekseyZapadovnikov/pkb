package mcp

import (
	"context"
	"log/slog"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func loggingMiddleware(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		logger := slog.Default().With("method", method, "date", time.Now().Format(time.DateOnly))
		switch req := req.(type) {
		case *sdk.CallToolRequest:
			if req.Params != nil {
				logger = logger.With("tool", req.Params.Name)
			}
		case *sdk.GetPromptRequest:
			if req.Params != nil {
				logger = logger.With("prompt", req.Params.Name)
			}
		case *sdk.ReadResourceRequest:
			if req.Params != nil {
				logger = logger.With("resource", req.Params.URI)
			}
		}
		logger.DebugContext(ctx, "MCP method started")

		start := time.Now()
		result, err := next(ctx, method, req)
		logger = logger.With("duration", time.Since(start))
		toolResult, _ := result.(*sdk.CallToolResult)

		switch {
		case err != nil:
			logger.ErrorContext(ctx, "MCP method failed", "error", err)
		case toolResult != nil && toolResult.IsError:
			logger.ErrorContext(ctx, "MCP tool failed", "is_error", true)
		default:
			logger.InfoContext(ctx, "MCP method completed")
		}
		return result, err
	}
}
