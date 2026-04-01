package model

import "testing"

func TestBuildVersionAwareURL(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		endpoint string
		want     string
	}{
		{
			name:     "anthropic default",
			base:     "https://api.anthropic.com",
			endpoint: "/v1/messages",
			want:     "https://api.anthropic.com/v1/messages",
		},
		{
			name:     "anthropic versioned proxy",
			base:     "https://api.z.ai/api/coding/paas/v4",
			endpoint: "/v1/messages",
			want:     "https://api.z.ai/api/coding/paas/v4/messages",
		},
		{
			name:     "openai versioned base",
			base:     "https://example.com/proxy/v1",
			endpoint: "/v1/chat/completions",
			want:     "https://example.com/proxy/v1/chat/completions",
		},
		{
			name:     "non versioned custom prefix",
			base:     "https://example.com/proxy/openai",
			endpoint: "/v1/responses",
			want:     "https://example.com/proxy/openai/v1/responses",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildVersionAwareURL(tt.base, tt.endpoint)
			if err != nil {
				t.Fatalf("buildVersionAwareURL() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("buildVersionAwareURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
