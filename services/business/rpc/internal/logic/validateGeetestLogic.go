package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateGeetestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateGeetestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateGeetestLogic {
	return &ValidateGeetestLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateGeetestLogic) ValidateGeetest(in *pb.ValidateGeetestRequest) (*pb.ValidateGeetestResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}

	if err := validateGeetest(l.ctx, l.svcCtx, geetestSceneValidate, in.LotNumber, in.CaptchaOutput, in.PassToken, in.GenTime, "", 0); err != nil {
		return nil, err
	}

	return &pb.ValidateGeetestResponse{
		Success: true,
		Message: "ok",
	}, nil
}
