package logic

import (
	"context"
	"internalwallet/proto/pb"
	"internalwallet/services/signer/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type LockHotWalletLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLockHotWalletLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LockHotWalletLogic {
	return &LockHotWalletLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LockHotWalletLogic) LockHotWallet(in *pb.LockHotWalletRequest) (*pb.LockHotWalletResponse, error) {
	seedID := defaultHotWalletSeedID
	if in != nil {
		seedID = normalizeSeedID(in.SeedId)
	}
	// 当前实现：一键锁定全部（足够满足“重启后需要解锁”的门禁语义）
	l.svcCtx.WalletRuntime.LockAll()
	return &pb.LockHotWalletResponse{
		Code:    0,
		Message: "success",
		SeedId:  seedID,
		Locked:  true,
	}, nil
}
