package logic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetCurrencyGlobalWithdrawAuditLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCurrencyGlobalWithdrawAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrencyGlobalWithdrawAuditLogic {
	return &GetCurrencyGlobalWithdrawAuditLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCurrencyGlobalWithdrawAuditLogic) GetCurrencyGlobalWithdrawAudit(in *pb.GetCurrencyGlobalWithdrawAuditRequest) (*pb.GetCurrencyGlobalWithdrawAuditResponse, error) {
	_ = in
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	var rows []*model.CurrencyGlobalWithdrawAuditRuleModel
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Order("chain_code ASC, sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		l.Logger.Errorf("list global withdraw audit rules failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	byChain := map[string][]*pb.CurrencyWithdrawAuditRuleItem{}
	for _, r := range rows {
		if r == nil {
			continue
		}
		cc := normalizeCode(r.ChainCode)
		byChain[cc] = append(byChain[cc], &pb.CurrencyWithdrawAuditRuleItem{
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

	// Prefer a stable chain list from the chains table, and also include any legacy/unknown chain codes in DB.
	seen := map[string]struct{}{}
	chainCodes := make([]string, 0, len(byChain))
	if l.svcCtx.ChainRepo != nil {
		if chains, err := l.svcCtx.ChainRepo.ListAll(l.ctx); err == nil && len(chains) > 0 {
			for _, c := range chains {
				if c == nil {
					continue
				}
				cc := normalizeCode(c.Name)
				if cc == "" {
					continue
				}
				if _, ok := seen[cc]; ok {
					continue
				}
				seen[cc] = struct{}{}
				chainCodes = append(chainCodes, cc)
			}
		}
	}
	for cc := range byChain {
		if cc == "" {
			continue
		}
		if _, ok := seen[cc]; ok {
			continue
		}
		seen[cc] = struct{}{}
		chainCodes = append(chainCodes, cc)
	}
	sort.Strings(chainCodes)

	respGroups := make([]*pb.CurrencyChainAuditRules, 0, len(chainCodes))
	for _, cc := range chainCodes {
		respGroups = append(respGroups, &pb.CurrencyChainAuditRules{
			ChainCode: cc,
			Rules:     byChain[cc],
		})
	}

	return &pb.GetCurrencyGlobalWithdrawAuditResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", len(rows)),
		Data: &pb.GetCurrencyGlobalWithdrawAuditData{
			AuditRules: respGroups,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
