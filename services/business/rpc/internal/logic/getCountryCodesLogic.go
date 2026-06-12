package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCountryCodesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCountryCodesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCountryCodesLogic {
	return &GetCountryCodesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取支持的国家/地区区号
func (l *GetCountryCodesLogic) GetCountryCodes(in *pb.GetCountryCodesRequest) (*pb.GetCountryCodesResponse, error) {
	return &pb.GetCountryCodesResponse{
		Success:      true,
		Message:      "ok",
		CountryCodes: l.svcCtx.SupportedCountryCodes,
	}, nil
}
