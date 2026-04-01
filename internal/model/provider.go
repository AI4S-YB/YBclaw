package model

import (
	"fmt"
	"strings"
)

type Provider string

const (
	ProviderAnthropic      Provider = "anthropic"
	ProviderOpenAIChat     Provider = "openai-chat"
	ProviderOpenAIResponse Provider = "openai-responses"
)

func ParseProvider(value string) (Provider, error) {
	switch normalized := strings.TrimSpace(strings.ToLower(value)); normalized {
	case "", string(ProviderAnthropic):
		return ProviderAnthropic, nil
	case string(ProviderOpenAIChat):
		return ProviderOpenAIChat, nil
	case "openai-response", string(ProviderOpenAIResponse):
		return ProviderOpenAIResponse, nil
	default:
		return "", fmt.Errorf("unsupported provider %q; expected anthropic, openai-chat, or openai-responses", value)
	}
}

func (p Provider) DefaultModel() string {
	switch p {
	case ProviderAnthropic:
		return "claude-sonnet-4-7"
	case ProviderOpenAIChat, ProviderOpenAIResponse:
		return "gpt-5.4"
	default:
		return ""
	}
}

func (p Provider) DefaultBaseURL() string {
	switch p {
	case ProviderAnthropic:
		return defaultAnthropicBaseURL
	case ProviderOpenAIChat, ProviderOpenAIResponse:
		return defaultOpenAIBaseURL
	default:
		return ""
	}
}

func (p Provider) APIKeyEnvVar() string {
	switch p {
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case ProviderOpenAIChat, ProviderOpenAIResponse:
		return "OPENAI_API_KEY"
	default:
		return ""
	}
}

func (p Provider) BaseURLEnvVar() string {
	switch p {
	case ProviderAnthropic:
		return "ANTHROPIC_BASE_URL"
	case ProviderOpenAIChat, ProviderOpenAIResponse:
		return "OPENAI_BASE_URL"
	default:
		return ""
	}
}

func NewClient(provider Provider, apiKey, baseURL string) (Client, error) {
	switch provider {
	case ProviderAnthropic:
		return NewAnthropicClient(apiKey, baseURL), nil
	case ProviderOpenAIChat:
		return NewOpenAIChatClient(apiKey, baseURL), nil
	case ProviderOpenAIResponse:
		return NewOpenAIResponsesClient(apiKey, baseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}
