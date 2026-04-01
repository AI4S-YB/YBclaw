package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAnthropicBaseURL = "https://api.anthropic.com"
	defaultAnthropicVersion = "2023-06-01"
)

type AnthropicClient struct {
	HTTPClient *http.Client
	BaseURL    string
	APIKey     string
	APIVersion string
}

func NewAnthropicClient(apiKey, baseURL string) *AnthropicClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultAnthropicBaseURL
	}
	return &AnthropicClient{
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
		BaseURL:    baseURL,
		APIKey:     apiKey,
		APIVersion: defaultAnthropicVersion,
	}
}

func (c *AnthropicClient) CreateMessage(ctx context.Context, req Request) (Response, error) {
	endpoint, err := buildMessagesURL(c.BaseURL)
	if err != nil {
		return Response{}, err
	}

	requestPayload := req
	requestPayload.Tools = normalizedToolDefinitions(req.Tools)

	body, err := json.Marshal(requestPayload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("anthropic-version", c.APIVersion)
	httpReq.Header.Set("x-api-key", c.APIKey)

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("request model: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return Response{}, parseAnthropicError(resp.StatusCode, respBody)
	}

	var out Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

func (c *AnthropicClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func buildMessagesURL(base string) (string, error) {
	return buildVersionAwareURL(base, "/v1/messages")
}

func parseAnthropicError(statusCode int, body []byte) error {
	type errorEnvelope struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	var envelope errorEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		return fmt.Errorf("anthropic API %d %s: %s", statusCode, envelope.Error.Type, envelope.Error.Message)
	}
	return fmt.Errorf("anthropic API %d: %s", statusCode, strings.TrimSpace(string(body)))
}
