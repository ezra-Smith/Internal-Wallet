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

type ListWalletProvidersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWalletProvidersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWalletProvidersLogic {
	return &ListWalletProvidersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWalletProvidersLogic) ListWalletProviders(in *pb.ListWalletProvidersReq) (*pb.ListWalletProvidersResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	items := []*pb.WalletProviderItem{}
	seen := map[string]struct{}{}
	if l.svcCtx.PlatformBindingRepository != nil {
		rows, _ := l.svcCtx.PlatformBindingRepository.ListBindingsByUser(l.ctx, uid)
		for _, r := range rows {
			parts := strings.SplitN(r.PlatformUid, ":", 2)
			if len(parts) != 2 {
				continue
			}
			provider := parts[0]
			if _, ok := seen[provider]; ok {
				continue
			}
			seen[provider] = struct{}{}
			items = append(items, &pb.WalletProviderItem{
				Id:      provider,
				Name:    provider,
				IconUrl: "",
				Type:    "web3",
			})
		}
	}
	return &pb.ListWalletProvidersResp{Success: true, Providers: items}, nil
}
