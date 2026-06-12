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

type UpdateCurrencyGlobalWithdrawAuditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCurrencyGlobalWithdrawAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCurrencyGlobalWithdrawAuditLogic {
	return &UpdateCurrencyGlobalWithdrawAuditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCurrencyGlobalWithdrawAuditLogic) UpdateCurrencyGlobalWithdrawAudit(in *pb.UpdateCurrencyGlobalWithdrawAuditRequest) (*pb.UpdateCurrencyGlobalWithdrawAuditResponse, error) {
	if in == nil {
		in = &pb.UpdateCurrencyGlobalWithdrawAuditRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyGlobalWithdrawAuditRuleRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	for _, group := range in.AuditRules {
		if group == nil || strings.TrimSpace(group.ChainCode) == "" {
			continue
		}
		chainCode := normalizeCode(group.ChainCode)

		enabledIntervals := make([]auditInterval, 0, len(group.Rules))
		rules := make([]*model.CurrencyGlobalWithdrawAuditRuleModel, 0, len(group.Rules))
		for _, r := range group.Rules {
			if r == nil {
				continue
			}
			strategy := strings.ToLower(strings.TrimSpace(r.Strategy))
			if !isValidWithdrawAuditStrategy(strategy) {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STRATEGY", "invalid strategy", map[string]string{"strategy": "invalid"})
			}

			minAmountStr := strings.TrimSpace(r.MinAmount)
			minD, ok := parseNonNegativeDecimal(minAmountStr)
			if !ok {
				l.Logger.Errorf("failed to parse min_amount for chain %s: %s", chainCode, minAmountStr)
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_AMOUNT",
					fmt.Sprintf("invalid min_amount for chain %s: cannot parse decimal", chainCode),
					map[string]string{"min_amount": minAmountStr, "chain_code": chainCode})
			}

			// Round to 6 decimal places (product standard)
			minD = roundToSixDecimals(minD)

			// Validate decimal range: decimal(40,6) allows max 34 integer digits and 6 decimal digits
			if !validateDecimalRange(minD) {
				l.Logger.Errorf("min_amount out of range for chain %s: %s (max 34 integer digits, 6 decimal digits)", chainCode, minD.String())
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AMOUNT_OUT_OF_RANGE",
					fmt.Sprintf("min_amount for chain %s exceeds maximum value (max 34 integer digits, 6 decimal digits)", chainCode),
					map[string]string{"min_amount": minD.String(), "chain_code": chainCode})
			}

			var maxPtr *string
			var maxDPtr *decimal.Decimal
			if s := strings.TrimSpace(r.MaxAmount); s != "" {
				maxD, ok := parseNonNegativeDecimal(s)
				if !ok {
					l.Logger.Errorf("failed to parse max_amount for chain %s: %s", chainCode, s)
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_AMOUNT",
						fmt.Sprintf("invalid max_amount for chain %s: cannot parse decimal", chainCode),
						map[string]string{"max_amount": s, "chain_code": chainCode})
				}

				// Round to 6 decimal places (product standard)
				maxD = roundToSixDecimals(maxD)

				// Validate decimal range
				if !validateDecimalRange(maxD) {
					l.Logger.Errorf("max_amount out of range for chain %s: %s (max 34 integer digits, 6 decimal digits)", chainCode, maxD.String())
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AMOUNT_OUT_OF_RANGE",
						fmt.Sprintf("max_amount for chain %s exceeds maximum value (max 34 integer digits, 6 decimal digits)", chainCode),
						map[string]string{"max_amount": maxD.String(), "chain_code": chainCode})
				}

				if !maxD.GreaterThan(minD) {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RANGE",
						fmt.Sprintf("max_amount must be > min_amount for chain %s", chainCode),
						map[string]string{"max_amount": maxD.String(), "min_amount": minD.String(), "chain_code": chainCode})
				}
				ss := maxD.String()
				maxPtr = &ss
				maxDPtr = &maxD
			}
			if r.Enabled {
				enabledIntervals = append(enabledIntervals, auditInterval{Min: minD, Max: maxDPtr})
			}

			rules = append(rules, &model.CurrencyGlobalWithdrawAuditRuleModel{
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

		if err := l.svcCtx.CurrencyGlobalWithdrawAuditRuleRepo.ReplaceForChain(l.ctx, chainCode, current.ID, rules); err != nil {
			l.Logger.Errorf("replace global withdraw audit rules failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
	}

	return &pb.UpdateCurrencyGlobalWithdrawAuditResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d chains)", len(in.AuditRules)),
		Data: &pb.UpdateCurrencyGlobalWithdrawAuditData{
			AuditRules: in.AuditRules,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
