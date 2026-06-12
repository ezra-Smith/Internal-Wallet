package logic

import (
	"context"
	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPreferencesLogic {
	return &GetPreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPreferencesLogic) GetPreferences(in *pb.GetPreferencesReq) (*pb.GetPreferencesResp, error) {
	uid := middleware.GetUserID(l.ctx)
	p, err := l.svcCtx.UserAccountRepository.GetByUID(l.ctx, uid)
	if err != nil {
		l.Errorf("get user account by uid failed: %v", err)
		return nil, err
	}
	resp := &pb.GetPreferencesResp{
		Success:   true,
		Language:  int32(p.Language),
		PriceUnit: int32(p.PriceUnit),
	}
	return resp, nil
}
