package okx

import (
	"internalwallet/services/swap/rpc/internal/config"
	"internalwallet/services/swap/rpc/internal/provider"
)

func init() {
	provider.RegisterProvider("okx", func(cfg config.ProviderConfig, chains []config.ChainConfig) (provider.SwapProvider, error) {
		return NewProvider(cfg.Okx, chains), nil
	})
	provider.RegisterProvider("okex", func(cfg config.ProviderConfig, chains []config.ChainConfig) (provider.SwapProvider, error) {
		return NewProvider(cfg.Okx, chains), nil
	})
}
