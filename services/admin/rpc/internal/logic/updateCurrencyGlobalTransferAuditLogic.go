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

type UpdateCurrencyGlobalTransferAuditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCurrencyGlobalTransferAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCurrencyGlobalTransferAuditLogic {
	return &UpdateCurrencyGlobalTransferAuditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCurrencyGlobalTransferAuditLogic) UpdateCurrencyGlobalTransferAudit(in *pb.UpdateCurrencyGlobalTransferAuditRequest) (*pb.UpdateCurrencyGlobalTransferAuditResponse, error) {
	if in == nil {
		in = &pb.UpdateCurrencyGlobalTransferAuditRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyGlobalTransferAuditRuleRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	// Parse and validate all rules grouped by asset
	rulesByAsset := make(map[string][]*model.CurrencyGlobalTransferAuditRuleModel)
	totalRules := 0

	for _, group := range in.AuditRules {
		if group == nil {
			continue
		}
		assetCode := normalizeCode(group.AssetCode)
		if assetCode == "" {
			continue
		}

		enabledIntervals := make([]auditInterval, 0, len(group.Rules))
		assetRules := make([]*model.CurrencyGlobalTransferAuditRuleModel, 0, len(group.Rules))

		for _, r := range group.Rules {
			if r == nil {
				continue
			}
			strategy := strings.ToLower(strings.TrimSpace(r.Strategy))
			if !isValidTransferAuditStrategy(strategy) {
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STRATEGY",
					fmt.Sprintf("invalid strategy for asset %s", assetCode), map[string]string{"strategy": "invalid", "asset_code": assetCode})
			}

			minAmountStr := strings.TrimSpace(r.MinAmount)
			minD, ok := parseNonNegativeDecimal(minAmountStr)
			if !ok {
				l.Logger.Errorf("failed to parse min_amount for asset %s: %s", assetCode, minAmountStr)
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MIN_AMOUNT",
					fmt.Sprintf("invalid min_amount for asset %s: cannot parse decimal", assetCode),
					map[string]string{"min_amount": minAmountStr, "asset_code": assetCode})
			}

			// Round to 6 decimal places (product standard)
			minD = roundToSixDecimals(minD)

			// Validate decimal range: decimal(40,6) allows max 34 integer digits and 6 decimal digits
			if !validateDecimalRange(minD) {
				l.Logger.Errorf("min_amount out of range for asset %s: %s (max 34 integer digits, 6 decimal digits)", assetCode, minD.String())
				return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AMOUNT_OUT_OF_RANGE",
					fmt.Sprintf("min_amount for asset %s exceeds maximum value (max 34 integer digits, 6 decimal digits)", assetCode),
					map[string]string{"min_amount": minD.String(), "asset_code": assetCode})
			}
			var maxPtr *string
			var maxDPtr *decimal.Decimal
			if s := strings.TrimSpace(r.MaxAmount); s != "" {
				maxD, ok := parseNonNegativeDecimal(s)
				if !ok {
					l.Logger.Errorf("failed to parse max_amount for asset %s: %s", assetCode, s)
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_MAX_AMOUNT",
						fmt.Sprintf("invalid max_amount for asset %s: cannot parse decimal", assetCode),
						map[string]string{"max_amount": s, "asset_code": assetCode})
				}

				// Round to 6 decimal places (product standard)
				maxD = roundToSixDecimals(maxD)

				// Validate decimal range
				if !validateDecimalRange(maxD) {
					l.Logger.Errorf("max_amount out of range for asset %s: %s (max 34 integer digits, 6 decimal digits)", assetCode, maxD.String())
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "AMOUNT_OUT_OF_RANGE",
						fmt.Sprintf("max_amount for asset %s exceeds maximum value (max 34 integer digits, 6 decimal digits)", assetCode),
						map[string]string{"max_amount": maxD.String(), "asset_code": assetCode})
				}

				if !maxD.GreaterThan(minD) {
					return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RANGE",
						fmt.Sprintf("max_amount must be > min_amount for asset %s", assetCode),
						map[string]string{"max_amount": maxD.String(), "min_amount": minD.String(), "asset_code": assetCode})
				}
				ss := maxD.String()
				maxPtr = &ss
				maxDPtr = &maxD
			}
			if r.Enabled {
				enabledIntervals = append(enabledIntervals, auditInterval{Min: minD, Max: maxDPtr})
			}

			assetRules = append(assetRules, &model.CurrencyGlobalTransferAuditRuleModel{
				AssetCode: assetCode,
				MinAmount: minD.String(),
				MaxAmount: maxPtr,
				Strategy:  strategy,
				Enabled:   r.Enabled,
				SortOrder: r.SortOrder,
				UpdatedBy: current.ID,
			})
		}

		// Validate non-overlapping for this asset
		if err := validateAuditIntervalsNonOverlapping(enabledIntervals); err != nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "OVERLAP",
				fmt.Sprintf("audit rules overlap for asset %s", assetCode), map[string]string{"audit_rules": err.Error(), "asset_code": assetCode})
		}

		rulesByAsset[assetCode] = assetRules
		totalRules += len(assetRules)
	}

	if err := l.svcCtx.CurrencyGlobalTransferAuditRuleRepo.ReplaceAll(l.ctx, current.ID, rulesByAsset); err != nil {
		l.Logger.Errorf("replace global transfer audit rules failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.UpdateCurrencyGlobalTransferAuditResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d rules, %d assets)", totalRules, len(rulesByAsset)),
		Data: &pb.UpdateCurrencyGlobalTransferAuditData{
			AuditRules: in.AuditRules,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
