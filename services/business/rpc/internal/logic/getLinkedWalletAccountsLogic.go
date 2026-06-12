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

type GetLinkedWalletAccountsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLinkedWalletAccountsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLinkedWalletAccountsLogic {
	return &GetLinkedWalletAccountsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetLinkedWalletAccountsLogic) GetLinkedWalletAccounts(in *pb.GetLinkedWalletAccountsReq) (*pb.GetLinkedWalletAccountsResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	items := []*pb.LinkedWalletAccountItem{}
	if l.svcCtx.PlatformBindingRepository != nil {
		rows, _ := l.svcCtx.PlatformBindingRepository.ListBindingsByUser(l.ctx, uid)
		for _, r := range rows {
			provider := ""
			account := ""
			parts := strings.SplitN(r.PlatformUid, ":", 2)
			if len(parts) == 2 {
				provider = parts[0]
				account = parts[1]
			}
			items = append(items, &pb.LinkedWalletAccountItem{
				BindingId:    r.BindingId,
				ProviderId:   provider,
				ProviderName: provider,
				AccountId:    account,
				AccountName:  account,
				AvatarUrl:    "",
				Balance:      "0",
				Currency:     "USD",
				IsPrimary:    r.Status == 2,
				BoundAt:      r.BoundAt.Format(time.RFC3339),
			})
		}
	}
	return &pb.GetLinkedWalletAccountsResp{Success: true, Accounts: items}, nil
}
