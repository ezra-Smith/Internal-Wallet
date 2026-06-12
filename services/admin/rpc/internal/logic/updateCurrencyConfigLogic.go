package logic

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type UpdateCurrencyConfigLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCurrencyConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCurrencyConfigLogic {
	return &UpdateCurrencyConfigLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCurrencyConfigLogic) UpdateCurrencyConfig(in *pb.UpdateCurrencyConfigRequest) (*pb.UpdateCurrencyConfigResponse, error) {
	if in == nil || strings.TrimSpace(in.AssetCode) == "" || in.Settings == nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"asset_code": "required",
			"settings":   "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AccountingRpc == nil || l.svcCtx.CurrencySettingsRepo == nil || l.svcCtx.CurrencyChainSettingsRepo == nil || l.svcCtx.CurrencyWithdrawFeeRuleRepo == nil || l.svcCtx.CurrencyWithdrawAuditRuleRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	assetCode := normalizeCode(in.AssetCode)
	accGetResp, callErr := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
	if callErr != nil {
		l.Logger.Errorf("call accounting GetAsset failed: %v", callErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if accGetResp == nil || !accGetResp.Success || accGetResp.Item == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "asset not found", map[string]string{"asset_code": "not found"})
	}
	asset := accGetResp.Item

	if in.IconUrl != nil {
		iconURL := strings.TrimSpace(in.IconUrl.GetValue())
		if iconURL != "" {
			if len(iconURL) > 2048 {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ICON_URL", "icon_url too long", map[string]string{"icon_url": "too long"})
			}
			u, parseErr := url.ParseRequestURI(iconURL)
			if parseErr != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ICON_URL", "invalid icon_url", map[string]string{"icon_url": "invalid"})
			}
		}
		accIconResp, callErr := l.svcCtx.AccountingRpc.UpdateAssetIcon(l.ctx, &pb.UpdateAssetIconRequest{
			Code:    assetCode,
			IconUrl: iconURL,
		})
		if callErr != nil {
			l.Logger.Errorf("call accounting UpdateAssetIcon failed: %v", callErr)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
		}
		if accIconResp == nil || !accIconResp.Success {
			return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "asset not found", map[string]string{"asset_code": "not found"})
		}
		asset.IconUrl = iconURL
	}

	// Load enabled chains for validation and name enrichment
	chainNameByCode := map[string]string{}
	if l.svcCtx.ChainRepo != nil {
		if chains, _ := l.svcCtx.ChainRepo.ListEnabled(l.ctx); len(chains) > 0 {
			for _, c := range chains {
				if c == nil {
					continue
				}
				code := normalizeCode(c.Name)
				chainNameByCode[code] = strings.TrimSpace(c.Network)
			}
		}
	}

	validateChain := func(code string) bool {
		if code == "" {
			return false
		}
		if len(chainNameByCode) == 0 {
			return true // fallback when chain repo unavailable
		}
		_, ok := chainNameByCode[code]
		return ok
	}

	// Enforce base currency invariant.
	web3SwapEnabled := enforceBaseCurrencySwapEnabled(assetCode, in.Settings.Web3SwapEnabled)

	if err := l.svcCtx.CurrencySettingsRepo.Upsert(l.ctx, &model.CurrencySettingsModel{
		AssetCode:              assetCode,
		Web2DepositEnabled:     in.Settings.Web2DepositEnabled,
		Web2WithdrawEnabled:    in.Settings.Web2WithdrawEnabled,
		Web2TransferEnabled:    in.Settings.Web2TransferEnabled,
		Web3DepositEnabled:     in.Settings.Web3DepositEnabled,
		Web3WithdrawEnabled:    in.Settings.Web3WithdrawEnabled,
		Web3SwapEnabled:        web3SwapEnabled,
		UseGlobalWithdrawFee:   in.Settings.UseGlobalWithdrawFee,
		UseGlobalWithdrawAudit: in.Settings.UseGlobalWithdrawAudit,
		UseGlobalTransferAudit: in.Settings.UseGlobalTransferAudit,
		UpdatedBy:              current.ID,
	}); err != nil {
		l.Logger.Errorf("upsert currency settings failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Replace chain mapping if provided.
	if in.Chains != nil {
		seen := map[string]struct{}{}
		chainItems := make([]*model.CurrencyChainSettingsModel, 0, len(in.Chains))
		for _, c := range in.Chains {
			if c == nil || strings.TrimSpace(c.ChainCode) == "" {
				continue
			}
			chainCode := normalizeCode(c.ChainCode)
			if _, dup := seen[chainCode]; dup {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "DUP_CHAIN", "duplicate chain_code", map[string]string{"chain_code": chainCode})
			}
			seen[chainCode] = struct{}{}
			if !validateChain(chainCode) {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CHAIN", "chain not found or disabled", map[string]string{"chain_code": chainCode})
			}
			status := c.Status
			if status != 1 && status != 2 {
				status = 1
			}
			var minWithdraw *string
			if s := strings.TrimSpace(c.MinWithdrawAmount); s != "" {
				if d, ok := parseNonNegativeDecimal(s); !ok {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_WITHDRAW", "invalid min_withdraw_amount", map[string]string{"min_withdraw_amount": "invalid"})
				} else {
					ss := d.String()
					minWithdraw = &ss
				}
			}
			var minDeposit *string
			if s := strings.TrimSpace(c.MinDepositAmount); s != "" {
				if d, ok := parseNonNegativeDecimal(s); !ok {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_DEPOSIT", "invalid min_deposit_amount", map[string]string{"min_deposit_amount": "invalid"})
				} else {
					ss := d.String()
					minDeposit = &ss
				}
			}
			var contractAddress *string
			if s := strings.TrimSpace(c.ContractAddress); s != "" {
				if strings.HasPrefix(strings.ToLower(s), "0x") && !validateEVMAddress(strings.ToLower(s)) {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CONTRACT_ADDRESS", "invalid contract_address", map[string]string{"contract_address": "invalid"})
				}
				ss := s
				contractAddress = &ss
			}

			// token_decimals (required when contract_address is set)
			var tokenDecimals *int32
			if contractAddress != nil {
				if c.TokenDecimals == nil {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "TOKEN_DECIMALS_REQUIRED", "token_decimals is required for token assets", map[string]string{
						"token_decimals": "required",
						"chain_code":     chainCode,
					})
				}
				v := c.TokenDecimals.GetValue()
				if v < 0 || v > 30 {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_TOKEN_DECIMALS", "invalid token_decimals (expected 0..30)", map[string]string{
						"token_decimals": fmt.Sprintf("%d", v),
						"chain_code":     chainCode,
					})
				}
				vv := v
				tokenDecimals = &vv
			}
			item := &model.CurrencyChainSettingsModel{
				AssetCode:       assetCode,
				ChainCode:       chainCode,
				DepositEnabled:  c.DepositEnabled,
				WithdrawEnabled: c.WithdrawEnabled,
				Web3AssetDisplayEnabled: func() bool {
					// Backward-compat: if client doesn't send this field (nil), default to true.
					if c.Web3AssetDisplayEnabled == nil {
						return true
					}
					return c.Web3AssetDisplayEnabled.GetValue()
				}(),
				ConsolidationEnabled: c.ConsolidationEnabled,
				Status:               status,
				ContractAddress:      contractAddress,
				TokenDecimals:        tokenDecimals,
				MinWithdrawAmount:    minWithdraw,
				MinDepositAmount:     minDeposit,
				UpdatedBy:            current.ID,
			}
			item.ID = c.Id
			chainItems = append(chainItems, item)
		}
		if err := l.svcCtx.CurrencyChainSettingsRepo.ReplaceForAsset(l.ctx, assetCode, current.ID, chainItems); err != nil {
			l.Logger.Errorf("replace currency chains failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
	}

	// Replace fee rules for provided chains.
	if in.FeeRules != nil {
		for _, group := range in.FeeRules {
			if group == nil || strings.TrimSpace(group.ChainCode) == "" {
				continue
			}
			chainCode := normalizeCode(group.ChainCode)
			if !validateChain(chainCode) {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CHAIN", "chain not found or disabled", map[string]string{"chain_code": chainCode})
			}
			rules := make([]*model.CurrencyWithdrawFeeRuleModel, 0, len(group.Rules))
			for _, r := range group.Rules {
				if r == nil {
					continue
				}
				rt := strings.ToLower(strings.TrimSpace(r.RuleType))
				if !isValidFeeRuleType(rt) {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RULE_TYPE", "invalid rule_type", map[string]string{"rule_type": "invalid"})
				}
				valStr := strings.TrimSpace(r.Value)
				val, ok := parseNonNegativeDecimal(valStr)
				if !ok {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_VALUE", "invalid value", map[string]string{"value": "invalid"})
				}
				if rt == "percent" && val.GreaterThan(decimal.NewFromInt(100)) {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PERCENT", "percent must be <= 100", map[string]string{"value": "invalid"})
				}

				var minFee *string
				if s := strings.TrimSpace(r.MinFee); s != "" {
					d, ok := parseNonNegativeDecimal(s)
					if !ok {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_FEE", "invalid min_fee", map[string]string{"min_fee": "invalid"})
					}
					d = roundToSixDecimals(d)
					if !validateDecimalRange(d) {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "FEE_OUT_OF_RANGE", "min_fee out of range", map[string]string{"min_fee": d.String()})
					}
					ss := d.String()
					minFee = &ss
				}
				var maxFee *string
				if s := strings.TrimSpace(r.MaxFee); s != "" {
					d, ok := parseNonNegativeDecimal(s)
					if !ok {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_FEE", "invalid max_fee", map[string]string{"max_fee": "invalid"})
					}
					d = roundToSixDecimals(d)
					if !validateDecimalRange(d) {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "FEE_OUT_OF_RANGE", "max_fee out of range", map[string]string{"max_fee": d.String()})
					}
					ss := d.String()
					maxFee = &ss
				}
				if minFee != nil && maxFee != nil {
					minD, _ := decimal.NewFromString(*minFee)
					maxD, _ := decimal.NewFromString(*maxFee)
					if minD.GreaterThan(maxD) {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_MAX", "min_fee must be <= max_fee", map[string]string{"min_fee": "invalid"})
					}
				}

				// Parse and validate min_amount
				var minAmount *string
				if s := strings.TrimSpace(r.MinAmount); s != "" {
					d, ok := parseNonNegativeDecimal(s)
					if !ok {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_AMOUNT", "invalid min_amount", map[string]string{"min_amount": "invalid"})
					}
					d = roundToSixDecimals(d)
					if !validateDecimalRange(d) {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AMOUNT_OUT_OF_RANGE", "min_amount out of range", map[string]string{"min_amount": d.String()})
					}
					ss := d.String()
					minAmount = &ss
				}

				// Parse and validate max_amount
				var maxAmount *string
				if s := strings.TrimSpace(r.MaxAmount); s != "" {
					d, ok := parseNonNegativeDecimal(s)
					if !ok {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_AMOUNT", "invalid max_amount", map[string]string{"max_amount": "invalid"})
					}
					d = roundToSixDecimals(d)
					if !validateDecimalRange(d) {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AMOUNT_OUT_OF_RANGE", "max_amount out of range", map[string]string{"max_amount": d.String()})
					}
					ss := d.String()
					maxAmount = &ss
				}

				// Validate min_amount < max_amount (注意：这里用 < 而不是 <=，因为区间是左闭右开)
				if minAmount != nil && maxAmount != nil {
					minD, _ := decimal.NewFromString(*minAmount)
					maxD, _ := decimal.NewFromString(*maxAmount)
					if minD.GreaterThanOrEqual(maxD) {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_AMOUNT_RANGE", "min_amount must be < max_amount", map[string]string{"min_amount": *minAmount, "max_amount": *maxAmount})
					}
				}

				rules = append(rules, &model.CurrencyWithdrawFeeRuleModel{
					AssetCode: assetCode,
					ChainCode: chainCode,
					RuleType:  rt,
					Value:     val.String(),
					MinFee:    minFee,
					MaxFee:    maxFee,
					MinAmount: minAmount,
					MaxAmount: maxAmount,
					Enabled:   r.Enabled,
					SortOrder: r.SortOrder,
					UpdatedBy: current.ID,
				})
			}
			if err := l.svcCtx.CurrencyWithdrawFeeRuleRepo.ReplaceForAssetChain(l.ctx, assetCode, chainCode, current.ID, rules); err != nil {
				l.Logger.Errorf("replace withdraw fee rules failed: %v", err)
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
			}
		}
	}

	// Replace withdraw audit rules for provided chains.
	if in.AuditRules != nil {
		for _, group := range in.AuditRules {
			if group == nil || strings.TrimSpace(group.ChainCode) == "" {
				continue
			}
			chainCode := normalizeCode(group.ChainCode)
			if !validateChain(chainCode) {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CHAIN", "chain not found or disabled", map[string]string{"chain_code": chainCode})
			}

			enabledIntervals := make([]auditInterval, 0, len(group.Rules))
			sortOrderSeen := make(map[int32]bool)
			rules := make([]*model.CurrencyWithdrawAuditRuleModel, 0, len(group.Rules))
			for _, r := range group.Rules {
				if r == nil {
					continue
				}

				// 检查 sort_order 是否重复
				if sortOrderSeen[r.SortOrder] {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "DUPLICATE_SORT_ORDER",
						fmt.Sprintf("不能定义重复的排序序号：%d", r.SortOrder),
						map[string]string{"sort_order": fmt.Sprintf("%d", r.SortOrder), "chain_code": chainCode})
				}
				sortOrderSeen[r.SortOrder] = true

				strategy := strings.ToLower(strings.TrimSpace(r.Strategy))
				if !isValidWithdrawAuditStrategy(strategy) {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STRATEGY", "invalid strategy", map[string]string{"strategy": "invalid"})
				}

				minD, ok := parseNonNegativeDecimal(strings.TrimSpace(r.MinAmount))
				if !ok {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_AMOUNT", "invalid min_amount", map[string]string{"min_amount": "invalid"})
				}
				var maxPtr *string
				var maxDPtr *decimal.Decimal
				if s := strings.TrimSpace(r.MaxAmount); s != "" {
					maxD, ok := parseNonNegativeDecimal(s)
					if !ok {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_AMOUNT", "invalid max_amount", map[string]string{"max_amount": "invalid"})
					}
					if !maxD.GreaterThan(minD) {
						return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RANGE", "max_amount must be > min_amount", map[string]string{"max_amount": "invalid"})
					}
					ss := maxD.String()
					maxPtr = &ss
					maxDPtr = &maxD
				}

				if r.Enabled {
					enabledIntervals = append(enabledIntervals, auditInterval{Min: minD, Max: maxDPtr})
				}
				rules = append(rules, &model.CurrencyWithdrawAuditRuleModel{
					AssetCode: assetCode,
					ChainCode: chainCode,
					MinAmount: minD.String(),
					MaxAmount: maxPtr,
					Strategy:  strategy,
					Enabled:   r.Enabled,
					SortOrder: r.SortOrder,
					UpdatedBy: current.ID,
				})
			}

			if err := validateAuditIntervalsNonOverlapping(enabledIntervals); err != nil {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "OVERLAP", "audit rules overlap", map[string]string{"audit_rules": err.Error()})
			}

			if err := l.svcCtx.CurrencyWithdrawAuditRuleRepo.ReplaceForAssetChain(l.ctx, assetCode, chainCode, current.ID, rules); err != nil {
				l.Logger.Errorf("replace withdraw audit rules failed: %v", err)
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
			}
		}
	}

	return &pb.UpdateCurrencyConfigResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%s)", assetCode),
		Data: &pb.UpdateCurrencyConfigData{
			Currency: &pb.CurrencyItem{
				AssetCode: assetCode,
				AssetName: func() string {
					if strings.TrimSpace(asset.Name) == "" {
						return assetCode
					}
					return strings.TrimSpace(asset.Name)
				}(),
				Status: asset.Status,
				IconUrl: func() string {
					return strings.TrimSpace(asset.IconUrl)
				}(),
			},
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil

}
