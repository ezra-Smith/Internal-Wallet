package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwapExecuteSwapLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapExecuteSwapLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapExecuteSwapLogic {
	return &SwapExecuteSwapLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SwapExecuteSwapLogic) SwapExecuteSwap(in *pb.BusinessSwapExecuteSwapRequest) (*pb.BusinessSwapExecuteSwapResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	if l.svcCtx.SwapRpc == nil {
		return nil, errx.SwapServiceNotAvailable()
	}
	if in.ChainId <= 0 {
		return nil, errx.InvalidChainId()
	}
	if strings.TrimSpace(in.WalletAddress) == "" {
		return nil, errx.InvalidWalletAddress()
	}
	if strings.TrimSpace(in.FromTokenAddress) == "" || strings.TrimSpace(in.ToTokenAddress) == "" {
		return nil, errx.InvalidTokenAddress()
	}
	if strings.TrimSpace(in.Amount) == "" {
		return nil, errx.InvalidAmount()
	}

	resp, err := l.svcCtx.SwapRpc.ExecuteSwap(l.ctx, &pb.ExecuteSwapRequest{
		ChainId:          in.ChainId,
		WalletAddress:    in.WalletAddress,
		FromTokenAddress: in.FromTokenAddress,
		ToTokenAddress:   in.ToTokenAddress,
		Amount:           in.Amount,
		SlippageBps:      in.SlippageBps,
		Recipient:        in.Recipient,
		CheckAllowance:   in.CheckAllowance,
		IncludeApproveTx: in.IncludeApproveTx,
		EstimatedGas:     in.EstimatedGas,
	})
	if err != nil {
		l.Errorf("swap execute failed: %v", err)
		return nil, errx.SwapServiceError()
	}
	if resp == nil {
		return nil, errx.SwapServiceError()
	}
	if !resp.Success {
		return nil, errx.Internal(resp.Message)
	}

	out := &pb.BusinessSwapExecuteSwapResponse{Success: true, Message: "ok"}
	if resp.Data != nil {
		out.Data = &pb.BusinessSwapExecuteSwapData{
			SwapId:    resp.Data.SwapId,
			Provider:  resp.Data.Provider,
			Quote:     mapSwapQuoteToBiz(resp.Data.Quote),
			ApproveTx: mapSwapUnsignedTxToBiz(resp.Data.ApproveTx),
			SwapTx:    mapSwapUnsignedTxToBiz(resp.Data.SwapTx),
		}
	}
	return out, nil
}
