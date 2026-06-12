package logic

import (
	"context"
	"fmt"
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

type UpdateCurrencyGlobalWithdrawFeeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCurrencyGlobalWithdrawFeeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCurrencyGlobalWithdrawFeeLogic {
	return &UpdateCurrencyGlobalWithdrawFeeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCurrencyGlobalWithdrawFeeLogic) UpdateCurrencyGlobalWithdrawFee(in *pb.UpdateCurrencyGlobalWithdrawFeeRequest) (*pb.UpdateCurrencyGlobalWithdrawFeeResponse, error) {
	if in == nil {
		in = &pb.UpdateCurrencyGlobalWithdrawFeeRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyGlobalWithdrawFeeRuleRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// Build a set of known chains for reference (best-effort validation).
	// Note: chain_code can be either a chain (ETH, BSC, TRON) or an asset code (USDT, USDC).
	// We only log warnings for unknown codes but do not block the operation.
	knownChains := map[string]struct{}{}
	if l.svcCtx.ChainRepo != nil {
		if chains, _ := l.svcCtx.ChainRepo.ListAll(l.ctx); len(chains) > 0 {
			for _, c := range chains {
				if c == nil {
					continue
				}
				cc := normalizeCode(c.Name)
				if cc == "" {
					continue
				}
				knownChains[cc] = struct{}{}
			}
		}
	}

	for _, group := range in.FeeRules {
		if group == nil || strings.TrimSpace(group.ChainCode) == "" {
			continue
		}
		chainCode := normalizeCode(group.ChainCode)
		if chainCode == "" {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CHAIN_CODE", "chain_code is required", map[string]string{"chain_code": "required"})
		}

		// Log warning if code is not in known chains (but allow it, as it might be an asset code)
		if len(knownChains) > 0 {
			if _, exists := knownChains[chainCode]; !exists {
				l.Logger.Infof("chain_code '%s' not found in chains table", chainCode)
			}
		}
		rules := make([]*model.CurrencyGlobalWithdrawFeeRuleModel, 0, len(group.Rules))
		for _, r := range group.Rules {
			if r == nil {
				continue
			}
			rt := strings.ToLower(strings.TrimSpace(r.RuleType))
			if !isValidFeeRuleType(rt) {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RULE_TYPE", "invalid rule_type", map[string]string{"rule_type": "invalid"})
			}

			// Parse and validate value
			valStr := strings.TrimSpace(r.Value)
			val, ok := parseNonNegativeDecimal(valStr)
			if !ok {
				l.Logger.Errorf("failed to parse value for chain %s: %s", chainCode, valStr)
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_VALUE",
					fmt.Sprintf("invalid value for chain %s: cannot parse decimal", chainCode),
					map[string]string{"value": valStr, "chain_code": chainCode})
			}

			// Round to 6 decimal places (product standard)
			val = roundToSixDecimals(val)

			// Validate decimal range
			if !validateDecimalRange(val) {
				l.Logger.Errorf("value out of range for chain %s: %s (max 34 integer digits, 6 decimal digits)", chainCode, val.String())
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "VALUE_OUT_OF_RANGE",
					fmt.Sprintf("value for chain %s exceeds maximum value (max 34 integer digits, 6 decimal digits)", chainCode),
					map[string]string{"value": val.String(), "chain_code": chainCode})
			}

			if rt == "percent" && val.GreaterThan(decimal.NewFromInt(100)) {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PERCENT", "percent must be <= 100", map[string]string{"value": val.String(), "chain_code": chainCode})
			}

			// Parse and validate min_fee
			var minFee *string
			if s := strings.TrimSpace(r.MinFee); s != "" {
				d, ok := parseNonNegativeDecimal(s)
				if !ok {
					l.Logger.Errorf("failed to parse min_fee for chain %s: %s", chainCode, s)
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_FEE",
						fmt.Sprintf("invalid min_fee for chain %s: cannot parse decimal", chainCode),
						map[string]string{"min_fee": s, "chain_code": chainCode})
				}

				// Round to 6 decimal places (product standard)
				d = roundToSixDecimals(d)

				// Validate decimal range
				if !validateDecimalRange(d) {
					l.Logger.Errorf("min_fee out of range for chain %s: %s (max 34 integer digits, 6 decimal digits)", chainCode, d.String())
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "FEE_OUT_OF_RANGE",
						fmt.Sprintf("min_fee for chain %s exceeds maximum value (max 34 integer digits, 6 decimal digits)", chainCode),
						map[string]string{"min_fee": d.String(), "chain_code": chainCode})
				}

				ss := d.String()
				minFee = &ss
			}

			// Parse and validate max_fee
			var maxFee *string
			if s := strings.TrimSpace(r.MaxFee); s != "" {
				d, ok := parseNonNegativeDecimal(s)
				if !ok {
					l.Logger.Errorf("failed to parse max_fee for chain %s: %s", chainCode, s)
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_FEE",
						fmt.Sprintf("invalid max_fee for chain %s: cannot parse decimal", chainCode),
						map[string]string{"max_fee": s, "chain_code": chainCode})
				}

				// Round to 6 decimal places (product standard)
				d = roundToSixDecimals(d)

				// Validate decimal range
				if !validateDecimalRange(d) {
					l.Logger.Errorf("max_fee out of range for chain %s: %s (max 34 integer digits, 6 decimal digits)", chainCode, d.String())
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "FEE_OUT_OF_RANGE",
						fmt.Sprintf("max_fee for chain %s exceeds maximum value (max 34 integer digits, 6 decimal digits)", chainCode),
						map[string]string{"max_fee": d.String(), "chain_code": chainCode})
				}

				ss := d.String()
				maxFee = &ss
			}

			// Validate min_fee <= max_fee
			if minFee != nil && maxFee != nil {
				minD, _ := decimal.NewFromString(*minFee)
				maxD, _ := decimal.NewFromString(*maxFee)
				if minD.GreaterThan(maxD) {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_MAX",
						fmt.Sprintf("min_fee must be <= max_fee for chain %s", chainCode),
						map[string]string{"min_fee": *minFee, "max_fee": *maxFee, "chain_code": chainCode})
				}
			}

			// Parse and validate min_amount
			var minAmount *string
			if s := strings.TrimSpace(r.MinAmount); s != "" {
				d, ok := parseNonNegativeDecimal(s)
				if !ok {
					l.Logger.Errorf("failed to parse min_amount for chain %s: %s", chainCode, s)
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_AMOUNT",
						fmt.Sprintf("invalid min_amount for chain %s: cannot parse decimal", chainCode),
						map[string]string{"min_amount": s, "chain_code": chainCode})
				}
				d = roundToSixDecimals(d)
				if !validateDecimalRange(d) {
					l.Logger.Errorf("min_amount out of range for chain %s: %s", chainCode, d.String())
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AMOUNT_OUT_OF_RANGE",
						fmt.Sprintf("min_amount for chain %s exceeds maximum value", chainCode),
						map[string]string{"min_amount": d.String(), "chain_code": chainCode})
				}
				ss := d.String()
				minAmount = &ss
			}

			// Parse and validate max_amount
			var maxAmount *string
			if s := strings.TrimSpace(r.MaxAmount); s != "" {
				d, ok := parseNonNegativeDecimal(s)
				if !ok {
					l.Logger.Errorf("failed to parse max_amount for chain %s: %s", chainCode, s)
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_AMOUNT",
						fmt.Sprintf("invalid max_amount for chain %s: cannot parse decimal", chainCode),
						map[string]string{"max_amount": s, "chain_code": chainCode})
				}
				d = roundToSixDecimals(d)
				if !validateDecimalRange(d) {
					l.Logger.Errorf("max_amount out of range for chain %s: %s", chainCode, d.String())
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AMOUNT_OUT_OF_RANGE",
						fmt.Sprintf("max_amount for chain %s exceeds maximum value", chainCode),
						map[string]string{"max_amount": d.String(), "chain_code": chainCode})
				}
				ss := d.String()
				maxAmount = &ss
			}

			// Validate min_amount <= max_amount
			if minAmount != nil && maxAmount != nil {
				minD, _ := decimal.NewFromString(*minAmount)
				maxD, _ := decimal.NewFromString(*maxAmount)
				if minD.GreaterThanOrEqual(maxD) {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_AMOUNT_RANGE",
						fmt.Sprintf("min_amount must be < max_amount for chain %s", chainCode),
						map[string]string{"min_amount": *minAmount, "max_amount": *maxAmount, "chain_code": chainCode})
				}
			}

			rules = append(rules, &model.CurrencyGlobalWithdrawFeeRuleModel{
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
		if err := l.svcCtx.CurrencyGlobalWithdrawFeeRuleRepo.ReplaceForChain(l.ctx, chainCode, current.ID, rules); err != nil {
			l.Logger.Errorf("replace global withdraw fee rules failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
	}

	return &pb.UpdateCurrencyGlobalWithdrawFeeResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d chains)", len(in.FeeRules)),
		Data: &pb.UpdateCurrencyGlobalWithdrawFeeData{
			FeeRules: in.FeeRules,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
