package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	openaisdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"pkb/internal/usecase/service"
	"pkb/internal/usecase/service/config"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	client openaisdk.Client
	cfg    config.ProviderConfig
}

func NewClient(cfg config.ProviderConfig, doer HTTPDoer) *Client {

	opts := make([]option.RequestOption, 0, 3)
	opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	opts = append(opts, option.WithAPIKey(cfg.APIKey))
	opts = append(opts, option.WithHTTPClient(doer))

	return &Client{
		client: openaisdk.NewClient(opts...),
		cfg:    cfg,
	}
}

func (c *Client) GenerateJSON(ctx context.Context, req service.GenerateRequest) (json.RawMessage, error) {

	messages := make([]openaisdk.ChatCompletionMessageParamUnion, 0, 2)
	messages = append(messages, openaisdk.SystemMessage(req.SystemText))
	messages = append(messages, openaisdk.UserMessage(req.UserText))

	completion, err := c.client.Chat.Completions.New(ctx, openaisdk.ChatCompletionNewParams{
		Model:    shared.ChatModel(c.cfg.Model),
		Messages: messages,
		ResponseFormat: openaisdk.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        "classification_result",
					Description: openaisdk.String("Classification result for one source message."),
					Schema:      classificationResultSchema,
					Strict:      openaisdk.Bool(true),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create openai chat completion: %w", err)
	}
	if len(completion.Choices) == 0 {
		return nil, errors.New("openai response has no choices")
	}

	choice := completion.Choices[0]
	if choice.FinishReason == "length" || choice.FinishReason == "content_filter" {
		return nil, fmt.Errorf("openai response incomplete: finish_reason=%s", choice.FinishReason)
	}
	if choice.Message.Refusal != "" {
		return nil, fmt.Errorf("openai refused request: %s", choice.Message.Refusal)
	}
	if choice.Message.Content == "" {
		return nil, errors.New("openai response has empty content")
	}

	var raw json.RawMessage
	if err := json.Unmarshal([]byte(choice.Message.Content), &raw); err != nil {
		return nil, fmt.Errorf("decode openai response content as json: %w", err)
	}

	return raw, nil
}

var classificationResultSchema = shared.FunctionParameters{
	"type": "object",
	"properties": map[string]any{
		"SourceMessageID": map[string]any{
			"type": "integer",
		},
		"Result": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"Kind": map[string]any{
						"type": "string",
						"enum": []string{"knowledge", "unknown"},
					},
					"Title": map[string]any{
						"type": "string",
					},
					"Body": map[string]any{
						"type": "string",
					},
					"TopicSlugs": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "string",
						},
					},
					"Confidence": map[string]any{
						"type": "number",
					},
					"DataJSON": map[string]any{
						"anyOf": []map[string]any{
							{
								"type": "null",
							},
							{
								"type": "object",
								"properties": map[string]any{
									"deadline": map[string]any{
										"type": []string{"string", "null"},
									},
									"tags": map[string]any{
										"anyOf": []map[string]any{
											{
												"type": "null",
											},
											{
												"type": "array",
												"items": map[string]any{
													"type": "string",
												},
											},
										},
									},
									"project": map[string]any{
										"type": []string{"string", "null"},
									},
									"technology": map[string]any{
										"type": []string{"string", "null"},
									},
									"priority": map[string]any{
										"type": []string{"string", "null"},
									},
									"source": map[string]any{
										"type": []string{"string", "null"},
									},
									"language": map[string]any{
										"type": []string{"string", "null"},
									},
									"status": map[string]any{
										"type": []string{"string", "null"},
									},
								},
								"required": []string{
									"deadline",
									"tags",
									"project",
									"technology",
									"priority",
									"source",
									"language",
									"status",
								},
								"additionalProperties": false,
							},
						},
					},
					"Reason": map[string]any{
						"type": "string",
					},
					"RawOutputJSON": map[string]any{
						"type": "null",
					},
				},
				"required": []string{
					"Kind",
					"Title",
					"Body",
					"TopicSlugs",
					"Confidence",
					"DataJSON",
					"Reason",
					"RawOutputJSON",
				},
				"additionalProperties": false,
			},
		},
	},
	"required": []string{
		"SourceMessageID",
		"Result",
	},
	"additionalProperties": false,
}
