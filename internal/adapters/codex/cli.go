package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"pkb/internal/usecase/service"
	"pkb/internal/usecase/service/config"
)

type Client struct {
	cfg config.ProviderConfig
}

func NewClient(cfg config.ProviderConfig) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) GenerateJSON(ctx context.Context, req service.GenerateRequest) (json.RawMessage, error) {
	outputFile, err := os.CreateTemp("", "pkb-codex-output-*.json")
	if err != nil {
		return nil, fmt.Errorf("create codex output file: %w", err)
	}
	outputPath := outputFile.Name()
	if err := outputFile.Close(); err != nil {
		return nil, fmt.Errorf("close codex output file: %w", err)
	}
	defer func() {
		_ = os.Remove(outputPath)
	}()

	args := []string{
		"-c", "features.plugins=false",
		"-c", "features.apps=false",
		"-c", "mcp_servers.github.enabled=false",

		"--ask-for-approval", "never",
		"exec",
		"--ephemeral",
		"--sandbox", "read-only",
		"--color", "never",
		"-o", outputPath,
	}
	if c.cfg.Model != "" {
		args = append(args, "-m", c.cfg.Model)
	}
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdin = strings.NewReader(buildPrompt(req))

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("run codex: %w: %s", err, strings.TrimSpace(string(output)))
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read codex output: %w", err)
	}
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, errors.New("codex response has empty content")
	}

	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode codex response as json: %w", err)
	}

	return raw, nil
}

func buildPrompt(req service.GenerateRequest) string {
	if req.SystemText == "" {
		return req.UserText
	}

	return req.SystemText + "\n\n" + req.UserText
}
