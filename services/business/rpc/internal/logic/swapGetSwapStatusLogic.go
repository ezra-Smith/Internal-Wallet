package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwapGetSwapStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapGetSwapStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapGetSwapStatusLogic {
	return &SwapGetSwapStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SwapGetSwapStatusLogic) SwapGetSwapStatus(in *pb.BusinessSwapGetSwapStatusRequest) (*pb.BusinessSwapGetSwapStatusResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	if l.svcCtx.SwapRpc == nil {
		return nil, errx.SwapServiceNotAvailable()
	}
	if in.ChainId <= 0 {
		return nil, errx.InvalidChainId()
	}
	if strings.TrimSpace(in.TxHash) == "" {
		return nil, errx.InvalidParam("invalid tx_hash")
	}

	resp, err := l.svcCtx.SwapRpc.GetSwapStatus(l.ctx, &pb.GetSwapStatusRequest{
		ChainId: in.ChainId,
		TxHash:  in.TxHash,
		SwapId:  in.SwapId,
	})
	if err != nil {
		l.Errorf("swap get status failed: %v", err)
		return nil, errx.SwapServiceError()
	}
	if resp == nil {
		return nil, errx.SwapServiceError()
	}
	if !resp.Success {
		return nil, errx.Internal(resp.Message)
	}

	out := &pb.BusinessSwapGetSwapStatusResponse{Success: true, Message: "ok"}
	if resp.Data != nil {
		out.Data = &pb.BusinessSwapStatusData{
			SwapId:                resp.Data.SwapId,
			ChainId:               resp.Data.ChainId,
			TxHash:                resp.Data.TxHash,
			Status:                mapSwapStatusToBiz(resp.Data.Status),
			Confirmations:         resp.Data.Confirmations,
			RequiredConfirmations: resp.Data.RequiredConfirmations,
			IsConfirmed:           resp.Data.IsConfirmed,
			BlockNumber:           resp.Data.BlockNumber,
			Provider:              resp.Data.Provider,
		}
	}
	return out, nil
}
