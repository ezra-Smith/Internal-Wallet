package logic

import (
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"
)

func normalizeAndValidateCountryCode(svcCtx *svc.ServiceContext, countryCode string) (string, error) {
	normalized, ok := svc.NormalizeCountryCode(countryCode)
	if !ok {
		return "", errx.InvalidCountryCode()
	}
	if svcCtx != nil && svcCtx.SupportedCountryCodeSet != nil {
		if _, exists := svcCtx.SupportedCountryCodeSet[normalized]; !exists {
			return "", errx.UnsupportedCountryCode()
		}
	}
	return normalized, nil
}
