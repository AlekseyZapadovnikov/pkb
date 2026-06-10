package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"pkb/internal/usecase/service"
	"pkb/internal/usecase/service/config"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	doer HTTPDoer
	cfg  config.ProviderConfig
}

func (c *Client) GenerateJSON(ctx context.Context, req service.GenerateRequest) (json.RawMessage, error) {

}
