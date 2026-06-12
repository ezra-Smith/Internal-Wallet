package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetBalanceValidationStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBalanceValidationStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBalanceValidationStatusLogic {
	return &GetBalanceValidationStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetBalanceValidationStatus 获取余额校验状态
func (l *GetBalanceValidationStatusLogic) GetBalanceValidationStatus(in *pb.GetBalanceValidationStatusReq) (*pb.GetBalanceValidationStatusResp, error) {
	// 检查余额校验器是否可用
	if l.svcCtx.BalanceValidator == nil {
		return &pb.GetBalanceValidationStatusResp{
			Success: false,
			Message: "Balance validator not initialized",
		}, nil
	}

	// 获取统计信息
	stats := l.svcCtx.BalanceValidator.GetStats()

	// 构建响应
	resp := &pb.GetBalanceValidationStatusResp{
		Success: true,
		Message: "Balance validation status retrieved successfully",
		Status: &pb.BalanceValidationStatus{
			IsEnabled:     l.svcCtx.BalanceValidator.IsEnabled(),
			IsRunning:     l.svcCtx.BalanceValidator.IsRunning(),
			TotalChecks:   stats.TotalChecks,
			MatchCount:    stats.MatchCount,
			MismatchCount: stats.MismatchCount,
			ErrorCount:    stats.ErrorCount,
			LastCheckTime: stats.LastCheckTime.Unix(),
		},
	}

	return resp, nil
}
