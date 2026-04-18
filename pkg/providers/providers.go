// Package providers abstracts LLM provider connections for AI-assisted analysis.
package providers

import (
	"context"
	"fmt"
)

// Provider is the interface implemented by all LLM backends.
type Provider interface {
	// Name returns the provider identifier.
	Name() string
	// Complete sends prompt to the LLM and returns the completion.
	Complete(ctx context.Context, prompt string) (string, error)
}

// ProviderType identifies a backend.
type ProviderType string

const (
	ProviderAnthropic ProviderType = "anthropic"
	ProviderOpenAI    ProviderType = "openai"
	ProviderLocal     ProviderType = "local"
)

// Config holds provider configuration.
type Config struct {
	Type   ProviderType `json:"type"`
	Model  string       `json:"model"`
	APIKey string       `json:"-"` // sourced from environment
}

// New returns a Provider for the given config.
func New(cfg Config) (Provider, error) {
	switch cfg.Type {
	case ProviderAnthropic:
		return &stubProvider{name: "anthropic", model: cfg.Model}, nil
	case ProviderOpenAI:
		return &stubProvider{name: "openai", model: cfg.Model}, nil
	case ProviderLocal:
		return &stubProvider{name: "local", model: cfg.Model}, nil
	default:
		return nil, fmt.Errorf("providers: unknown type %q", cfg.Type)
	}
}

type stubProvider struct {
	name  string
	model string
}

func (p *stubProvider) Name() string { return p.name }

func (p *stubProvider) Complete(_ context.Context, _ string) (string, error) {
	return fmt.Sprintf("[%s/%s stub response]", p.name, p.model), nil
}
