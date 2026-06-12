package oneinch

import (
	"internalwallet/services/swap/rpc/internal/config"
	"internalwallet/services/swap/rpc/internal/provider"
)

func init() {
	provider.RegisterProvider("1inch", func(cfg config.ProviderConfig, chains []config.ChainConfig) (provider.SwapProvider, error) {
		return NewProvider(cfg.OneInch, chains), nil
	})
	provider.RegisterProvider("oneinch", func(cfg config.ProviderConfig, chains []config.ChainConfig) (provider.SwapProvider, error) {
		return NewProvider(cfg.OneInch, chains), nil
	})
}
