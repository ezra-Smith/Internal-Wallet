package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwapGetExternalSwapStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapGetExternalSwapStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapGetExternalSwapStatusLogic {
	return &SwapGetExternalSwapStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SwapGetExternalSwapStatusLogic) SwapGetExternalSwapStatus(in *pb.BusinessSwapGetExternalSwapStatusRequest) (*pb.BusinessSwapGetExternalSwapStatusResponse, error) {
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

	// 调用 Swap RPC 的 GetExternalSwapStatus 方法
	resp, err := l.svcCtx.SwapRpc.GetExternalSwapStatus(l.ctx, &pb.GetExternalSwapStatusRequest{
		ChainId:         in.ChainId,
		TxHash:          in.TxHash,
		IsFromMyProject: in.IsFromMyProject,
	})
	if err != nil {
		l.Errorf("swap get external status failed: %v", err)
		return nil, errx.SwapServiceError()
	}
	if resp == nil {
		return nil, errx.SwapServiceError()
	}
	if !resp.Success {
		return nil, errx.Internal(resp.Message)
	}

	// 构建响应
	out := &pb.BusinessSwapGetExternalSwapStatusResponse{
		Success: true,
		Message: "ok",
	}

	// 映射数据
	if resp.Data != nil {
		out.Data = mapChainSwapHistoryItemToBusiness(resp.Data)
	}

	return out, nil
}

// mapChainSwapHistoryItemToBusiness 将 Swap 服务的 ChainSwapHistoryItem 映射为 Business 版本
func mapChainSwapHistoryItemToBusiness(item *pb.ChainSwapHistoryItem) *pb.BusinessSwapChainSwapHistoryItem {
	if item == nil {
		return nil
	}

	// 映射 from_tokens
	fromTokens := make([]*pb.BusinessSwapTokenDetail, 0, len(item.FromTokens))
	for _, token := range item.FromTokens {
		if token != nil {
			fromTokens = append(fromTokens, &pb.BusinessSwapTokenDetail{
				Symbol:       token.Symbol,
				Amount:       token.Amount,
				TokenAddress: token.TokenAddress,
			})
		}
	}

	// 映射 to_tokens
	toTokens := make([]*pb.BusinessSwapTokenDetail, 0, len(item.ToTokens))
	for _, token := range item.ToTokens {
		if token != nil {
			toTokens = append(toTokens, &pb.BusinessSwapTokenDetail{
				Symbol:       token.Symbol,
				Amount:       token.Amount,
				TokenAddress: token.TokenAddress,
			})
		}
	}

	return &pb.BusinessSwapChainSwapHistoryItem{
		ChainIndex:     item.ChainIndex,
		TxHash:         item.TxHash,
		BlockHeight:    item.BlockHeight,
		TxTime:         item.TxTime,
		Status:         mapChainSwapStatusToBusiness(item.Status),
		TxType:         item.TxType,
		FromAddress:    item.FromAddress,
		DexRouter:      item.DexRouter,
		ToAddress:      item.ToAddress,
		FromTokens:     fromTokens,
		ToTokens:       toTokens,
		ReferralAmount: item.ReferralAmount,
		ErrorMsg:       item.ErrorMsg,
		GasLimit:       item.GasLimit,
		GasUsed:        item.GasUsed,
		GasPrice:       item.GasPrice,
		TxFee:          item.TxFee,
	}
}

// mapChainSwapStatusToBusiness 映射链上交易状态
func mapChainSwapStatusToBusiness(status pb.ChainSwapStatus) pb.BusinessSwapChainSwapStatus {
	switch status {
	case pb.ChainSwapStatus_CHAIN_SWAP_STATUS_PENDING:
		return pb.BusinessSwapChainSwapStatus_BUSINESS_CHAIN_SWAP_STATUS_PENDING
	case pb.ChainSwapStatus_CHAIN_SWAP_STATUS_SUCCESS:
		return pb.BusinessSwapChainSwapStatus_BUSINESS_CHAIN_SWAP_STATUS_SUCCESS
	case pb.ChainSwapStatus_CHAIN_SWAP_STATUS_FAILED:
		return pb.BusinessSwapChainSwapStatus_BUSINESS_CHAIN_SWAP_STATUS_FAILED
	default:
		return pb.BusinessSwapChainSwapStatus_BUSINESS_CHAIN_SWAP_STATUS_UNSPECIFIED
	}
}
