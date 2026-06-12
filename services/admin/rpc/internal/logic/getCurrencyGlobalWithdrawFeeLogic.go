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

type GetCurrencyGlobalWithdrawFeeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCurrencyGlobalWithdrawFeeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrencyGlobalWithdrawFeeLogic {
	return &GetCurrencyGlobalWithdrawFeeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCurrencyGlobalWithdrawFeeLogic) GetCurrencyGlobalWithdrawFee(in *pb.GetCurrencyGlobalWithdrawFeeRequest) (*pb.GetCurrencyGlobalWithdrawFeeResponse, error) {
	_ = in
	if l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}
	if _, ok := admininterceptor.GetCurrentAdmin(l.ctx); !ok {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	var rows []*model.CurrencyGlobalWithdrawFeeRuleModel
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Order("chain_code ASC, sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		l.Logger.Errorf("list global withdraw fee rules failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	byChain := map[string][]*pb.CurrencyWithdrawFeeRuleItem{}
	for _, r := range rows {
		if r == nil {
			continue
		}
		cc := normalizeCode(r.ChainCode)
		byChain[cc] = append(byChain[cc], &pb.CurrencyWithdrawFeeRuleItem{
			Id:       r.ID,
			RuleType: strings.ToLower(strings.TrimSpace(r.RuleType)),
			Value:    strings.TrimSpace(r.Value),
			MinFee: func() string {
				if r.MinFee == nil {
					return ""
				}
				return strings.TrimSpace(*r.MinFee)
			}(),
			MaxFee: func() string {
				if r.MaxFee == nil {
					return ""
				}
				return strings.TrimSpace(*r.MaxFee)
			}(),
			MinAmount: func() string {
				if r.MinAmount == nil {
					return ""
				}
				return strings.TrimSpace(*r.MinAmount)
			}(),
			MaxAmount: func() string {
				if r.MaxAmount == nil {
					return ""
				}
				return strings.TrimSpace(*r.MaxAmount)
			}(),
			Enabled:   r.Enabled,
			SortOrder: r.SortOrder,
		})
	}

	// Prefer returning a stable chain list from the chains table (like GetCurrencyConfig does),
	// and also include any legacy/unknown chain codes that already exist in the rules table.
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
	respGroups := make([]*pb.CurrencyChainFeeRules, 0, len(chainCodes))
	for _, cc := range chainCodes {
		respGroups = append(respGroups, &pb.CurrencyChainFeeRules{
			ChainCode: cc,
			Rules:     byChain[cc],
		})
	}

	return &pb.GetCurrencyGlobalWithdrawFeeResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", len(rows)),
		Data: &pb.GetCurrencyGlobalWithdrawFeeData{
			FeeRules: respGroups,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
