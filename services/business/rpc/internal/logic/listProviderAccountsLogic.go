package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListProviderAccountsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListProviderAccountsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListProviderAccountsLogic {
	return &ListProviderAccountsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListProviderAccountsLogic) ListProviderAccounts(in *pb.ListProviderAccountsReq) (*pb.ListProviderAccountsResp, error) {
	if in == nil || strings.TrimSpace(in.ProviderId) == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	items := []*pb.ProviderAccountItem{}
	if l.svcCtx.PlatformBindingRepository != nil {
		rows, _ := l.svcCtx.PlatformBindingRepository.ListProviderAccountsByUser(l.ctx, uid, strings.TrimSpace(in.ProviderId))
		for _, r := range rows {
			parts := strings.SplitN(r.PlatformUid, ":", 2)
			account := ""
			if len(parts) == 2 {
				account = parts[1]
			}
			items = append(items, &pb.ProviderAccountItem{
				ProviderId:  strings.TrimSpace(in.ProviderId),
				AccountId:   account,
				AccountName: account,
				AvatarUrl:   "",
				Balance:     "0",
				Currency:    "USD",
			})
		}
	}
	return &pb.ListProviderAccountsResp{Success: true, Accounts: items}, nil
}
