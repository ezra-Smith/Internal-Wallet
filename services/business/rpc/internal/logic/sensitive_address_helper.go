package logic

import (
	"context"
	"strings"

	"internalwallet/services/business/rpc/internal/svc"
)

type SensitiveAddressCheckResult struct {
	IsBlacklisted bool
	Message       string
	Network       string
	RiskLevel     string
}

// CheckSensitiveAddressBlacklist checks whether a (chain,address) hits the on-chain sensitive address library.
//
// Notes:
// - The blacklist is keyed by `blacklist_addresses.network` (normalized lowercase), derived from `chain.network`.
// - When monitor_status is "stopped", it is treated as non-blocking.
func CheckSensitiveAddressBlacklist(ctx context.Context, svcCtx *svc.ServiceContext, chainCode string, address string) (*SensitiveAddressCheckResult, error) {
	if svcCtx == nil || svcCtx.SensitiveAddressRepository == nil {
		return &SensitiveAddressCheckResult{IsBlacklisted: false}, nil
	}
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	address = strings.TrimSpace(address)
	if chainCode == "" || address == "" {
		return &SensitiveAddressCheckResult{IsBlacklisted: false}, nil
	}

	network := strings.ToLower(chainCode)
	if svcCtx.ChainRepository != nil {
		if ch, err := svcCtx.ChainRepository.FindByName(ctx, chainCode); err == nil && ch != nil {
			if v := strings.TrimSpace(ch.Network); v != "" {
				network = strings.ToLower(v)
			}
		}
	}

	info, err := svcCtx.SensitiveAddressRepository.FindActive(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return &SensitiveAddressCheckResult{IsBlacklisted: false}, nil
	}

	if strings.EqualFold(strings.TrimSpace(info.MonitorStatus), "stopped") || strings.EqualFold(strings.TrimSpace(info.MonitorStatus), "disabled") {
		return &SensitiveAddressCheckResult{IsBlacklisted: false}, nil
	}

	msg := "目标地址命中链上敏感地址库，已阻断提交"
	if strings.TrimSpace(info.Reason) != "" {
		msg += "：" + strings.TrimSpace(info.Reason)
	}

	return &SensitiveAddressCheckResult{
		IsBlacklisted: true,
		Message:       msg,
		Network:       network,
		RiskLevel:     strings.TrimSpace(info.RiskLevel),
	}, nil
}
