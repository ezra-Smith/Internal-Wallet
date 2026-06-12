package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/provider"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetExternalSwapStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetExternalSwapStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetExternalSwapStatusLogic {
	return &GetExternalSwapStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetExternalSwapStatusLogic) GetExternalSwapStatus(in *pb.GetExternalSwapStatusRequest) (*pb.GetExternalSwapStatusResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "chain_id is required")
	}
	in.TxHash = strings.TrimSpace(in.TxHash)
	if in.TxHash == "" {
		return nil, status.Error(codes.InvalidArgument, "tx_hash is required")
	}

	if l.svcCtx.Provider == nil {
		return nil, status.Error(codes.Internal, "swap provider not ready")
	}

	// Check if provider supports extended methods
	extProvider, ok := l.svcCtx.Provider.(provider.ExtendedProvider)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "provider does not support external swap status")
	}

	result, err := extProvider.GetExternalSwapStatus(l.ctx, in.ChainId, in.TxHash, in.IsFromMyProject)
	if err != nil {
		return nil, err
	}

	return &pb.GetExternalSwapStatusResponse{
		Success: true,
		Message: "ok",
		Data:    mapChainSwapHistoryItemToPB(result),
	}, nil
}

func mapChainSwapHistoryItemToPB(item *provider.ChainSwapHistoryItem) *pb.ChainSwapHistoryItem {
	if item == nil {
		return nil
	}

	fromTokens := make([]*pb.TokenDetail, 0, len(item.FromTokens))
	for _, t := range item.FromTokens {
		fromTokens = append(fromTokens, &pb.TokenDetail{
			Symbol:       t.Symbol,
			Amount:       t.Amount,
			TokenAddress: t.TokenAddress,
		})
	}

	toTokens := make([]*pb.TokenDetail, 0, len(item.ToTokens))
	for _, t := range item.ToTokens {
		toTokens = append(toTokens, &pb.TokenDetail{
			Symbol:       t.Symbol,
			Amount:       t.Amount,
			TokenAddress: t.TokenAddress,
		})
	}

	return &pb.ChainSwapHistoryItem{
		ChainIndex:     item.ChainIndex,
		TxHash:         item.TxHash,
		BlockHeight:    item.BlockHeight,
		TxTime:         item.TxTime,
		Status:         mapChainSwapStatusToProto(item.Status),
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

func mapChainSwapStatusToProto(status provider.ChainSwapStatus) pb.ChainSwapStatus {
	switch status {
	case provider.ChainSwapStatusPending:
		return pb.ChainSwapStatus_CHAIN_SWAP_STATUS_PENDING
	case provider.ChainSwapStatusSuccess:
		return pb.ChainSwapStatus_CHAIN_SWAP_STATUS_SUCCESS
	case provider.ChainSwapStatusFailed:
		return pb.ChainSwapStatus_CHAIN_SWAP_STATUS_FAILED
	default:
		return pb.ChainSwapStatus_CHAIN_SWAP_STATUS_UNSPECIFIED
	}
}
