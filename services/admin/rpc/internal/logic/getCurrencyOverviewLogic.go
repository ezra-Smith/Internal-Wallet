package logic

import (
	"context"
	"fmt"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetCurrencyOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCurrencyOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCurrencyOverviewLogic {
	return &GetCurrencyOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetCurrencyOverviewLogic) GetCurrencyOverview(in *pb.GetCurrencyOverviewRequest) (*pb.GetCurrencyOverviewResponse, error) {
	_ = in
	if l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting rpc not configured", nil)
	}

	accResp, err := l.svcCtx.AccountingRpc.GetAssetOverview(l.ctx, &pb.Empty{})
	if err != nil {
		l.Logger.Errorf("call accounting GetAssetOverview failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if accResp == nil || !accResp.Success {
		msg := ""
		if accResp != nil {
			msg = accResp.Message
		}
		l.Logger.Errorf("accounting GetAssetOverview not success: %s", msg)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	return &pb.GetCurrencyOverviewResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", accResp.Total),
		Data: &pb.GetCurrencyOverviewData{
			Total:        accResp.Total,
			Enabled:      accResp.Enabled,
			Disabled:     accResp.Disabled,
			BaseCurrency: baseCurrencyCode,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
