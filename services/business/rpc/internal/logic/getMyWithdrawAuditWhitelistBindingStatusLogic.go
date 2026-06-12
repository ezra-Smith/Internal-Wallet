package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetMyWithdrawAuditWhitelistBindingStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMyWithdrawAuditWhitelistBindingStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyWithdrawAuditWhitelistBindingStatusLogic {
	return &GetMyWithdrawAuditWhitelistBindingStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMyWithdrawAuditWhitelistBindingStatusLogic) GetMyWithdrawAuditWhitelistBindingStatus(in *pb.GetMyWithdrawAuditWhitelistBindingStatusReq) (*pb.GetMyWithdrawAuditWhitelistBindingStatusResp, error) {
	_ = in
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	userID, _ := strconv.ParseInt(uidStr, 10, 64)
	if userID <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	if l.svcCtx == nil || l.svcCtx.UserWithdrawAuditWhitelistRuleRepository == nil {
		return nil, errx.Internal("db unavailable")
	}

	rows, err := l.svcCtx.UserWithdrawAuditWhitelistRuleRepository.ListActiveByUserSource(l.ctx, userID, "user")
	if err != nil {
		return nil, errx.DBError()
	}

	// 只对本接口做“链+地址”唯一性处理（不包含代币维度）
	type agg struct {
		item      *pb.WithdrawAuditWhitelistBindingStatusItem
		updatedAt time.Time
	}
	byChainAddr := make(map[string]agg, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		chain := strings.TrimSpace(r.ChainCode)
		addr := strings.TrimSpace(r.Address)
		if chain == "" || addr == "" {
			continue
		}

		// EVM 地址以 0x 开头，按不区分大小写做唯一化；Tron/Base58 等保持原样。
		keyAddr := addr
		if strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X") {
			keyAddr = strings.ToLower(addr)
		}
		key := strings.ToLower(chain) + "|" + keyAddr

		src := strings.TrimSpace(r.Source)
		if src == "" {
			src = "admin"
		}
		cur := agg{
			item: &pb.WithdrawAuditWhitelistBindingStatusItem{
				ChainCode:  chain,
				Address:    addr,
				WalletName: strings.TrimSpace(r.WalletName),
				WalletIcon: strings.TrimSpace(r.WalletIcon),
				Enabled:    r.Enabled,
				UpdatedAt:  r.UpdatedAt.Format(time.RFC3339),
				Source:     src,
			},
			updatedAt: r.UpdatedAt,
		}

		prev, ok := byChainAddr[key]
		if !ok {
			byChainAddr[key] = cur
			continue
		}
		// 同一链+地址出现多条时，优先取更新时间更晚的；同一时间则优先 enabled=true。
		if cur.updatedAt.After(prev.updatedAt) || (cur.updatedAt.Equal(prev.updatedAt) && cur.item.Enabled && !prev.item.Enabled) {
			byChainAddr[key] = cur
		}
	}

	items := make([]*pb.WithdrawAuditWhitelistBindingStatusItem, 0, len(byChainAddr))
	for _, v := range byChainAddr {
		items = append(items, v.item)
	}
	return &pb.GetMyWithdrawAuditWhitelistBindingStatusResp{
		Success: true,
		Message: "ok",
		Items:   items,
	}, nil
}
