package logic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetCurrencyGlobalTransferAuditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCurrencyGlobalTransferAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrencyGlobalTransferAuditLogic {
	return &GetCurrencyGlobalTransferAuditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCurrencyGlobalTransferAuditLogic) GetCurrencyGlobalTransferAudit(in *pb.GetCurrencyGlobalTransferAuditRequest) (*pb.GetCurrencyGlobalTransferAuditResponse, error) {
	_ = in
	if l.svcCtx.DB == nil || l.svcCtx.CurrencyGlobalTransferAuditRuleRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	// 注意：此方法允许内部服务调用（business-rpc），认证已在拦截器中处理
	// 如果是内部服务调用，拦截器会直接放行；如果是管理员调用，拦截器会验证 token

	rows, err := l.svcCtx.CurrencyGlobalTransferAuditRuleRepo.ListAll(l.ctx)
	if err != nil {
		l.Logger.Errorf("list global transfer audit rules failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Group rules by asset_code
	byAsset := map[string][]*pb.CurrencyTransferAuditRuleItem{}
	for _, r := range rows {
		if r == nil {
			continue
		}
		ac := normalizeCode(r.AssetCode)
		byAsset[ac] = append(byAsset[ac], &pb.CurrencyTransferAuditRuleItem{
			Id:        r.ID,
			MinAmount: strings.TrimSpace(r.MinAmount),
			MaxAmount: func() string {
				if r.MaxAmount == nil {
					return ""
				}
				return strings.TrimSpace(*r.MaxAmount)
			}(),
			Strategy:  strings.ToLower(strings.TrimSpace(r.Strategy)),
			Enabled:   r.Enabled,
			SortOrder: r.SortOrder,
		})
	}

	// Get all asset codes from Accounting service for a stable list
	seen := map[string]struct{}{}
	assetCodes := make([]string, 0, len(byAsset))
	if l.svcCtx.AccountingRpc != nil {
		accResp, callErr := l.svcCtx.AccountingRpc.ListAssets(l.ctx, &pb.ListAssetsRequest{
			Page:     1,
			PageSize: 1000, // Get all assets
		})
		if callErr == nil && accResp != nil {
			for _, asset := range accResp.Items {
				if asset == nil {
					continue
				}
				ac := normalizeCode(asset.Code)
				if ac == "" {
					continue
				}
				if _, ok := seen[ac]; ok {
					continue
				}
				seen[ac] = struct{}{}
				assetCodes = append(assetCodes, ac)
			}
		}
	}
	// Also include any asset codes that have rules but might not be in assets table
	for ac := range byAsset {
		if ac == "" {
			continue
		}
		if _, ok := seen[ac]; ok {
			continue
		}
		seen[ac] = struct{}{}
		assetCodes = append(assetCodes, ac)
	}
	sort.Strings(assetCodes)

	respGroups := make([]*pb.CurrencyAssetTransferAuditRules, 0, len(assetCodes))
	for _, ac := range assetCodes {
		respGroups = append(respGroups, &pb.CurrencyAssetTransferAuditRules{
			AssetCode: ac,
			Rules:     byAsset[ac],
		})
	}

	return &pb.GetCurrencyGlobalTransferAuditResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d rules, %d assets)", len(rows), len(assetCodes)),
		Data: &pb.GetCurrencyGlobalTransferAuditData{
			AuditRules: respGroups,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
