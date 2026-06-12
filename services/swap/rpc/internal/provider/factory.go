package provider

import (
	"fmt"
	"strings"

	"internalwallet/services/swap/rpc/internal/config"
)

type Constructor func(cfg config.ProviderConfig, chains []config.ChainConfig) (SwapProvider, error)

var registry = map[string]Constructor{}

// RegisterProvider registers a provider constructor by type.
// Providers should register themselves from their own package init().
func RegisterProvider(providerType string, ctor Constructor) {
	providerType = strings.ToLower(strings.TrimSpace(providerType))
	if providerType == "" || ctor == nil {
		return
	}
	registry[providerType] = ctor
}

func NewProvider(cfg config.ProviderConfig, chains []config.ChainConfig) (SwapProvider, error) {
	t := strings.ToLower(strings.TrimSpace(cfg.Type))
	if t == "" {
		t = "1inch"
	}
	if ctor, ok := registry[t]; ok {
		return ctor(cfg, chains)
	}
	return nil, fmt.Errorf("unsupported provider: %s", cfg.Type)
}
