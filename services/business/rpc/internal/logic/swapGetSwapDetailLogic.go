package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SwapGetSwapDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapGetSwapDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapGetSwapDetailLogic {
	return &SwapGetSwapDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SwapGetSwapDetailLogic) SwapGetSwapDetail(in *pb.BusinessSwapGetSwapDetailRequest) (*pb.BusinessSwapGetSwapDetailResponse, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	if l.svcCtx.SwapRpc == nil {
		return nil, errx.SwapServiceNotAvailable()
	}
	txHash := strings.TrimSpace(in.TxHash)
	if txHash == "" {
		return nil, errx.InvalidParam("invalid tx_hash")
	}

	resp, err := l.svcCtx.SwapRpc.GetSwapDetail(l.ctx, &pb.GetSwapDetailRequest{
		ChainId: in.ChainId,
		TxHash:  txHash,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return nil, errx.NotFound("swap transaction not found")
		}
		l.Errorf("swap get detail failed: %v", err)
		return nil, errx.SwapServiceError()
	}
	if resp == nil {
		return nil, errx.SwapServiceError()
	}
	if !resp.Success {
		return nil, errx.Internal(resp.Message)
	}

	out := &pb.BusinessSwapGetSwapDetailResponse{Success: true, Message: "ok"}
	if resp.Data != nil {
		out.Data = &pb.BusinessSwapDetailData{
			SwapId:        strconv.FormatInt(resp.Data.Id, 10),
			ApiKeyId:      resp.Data.ApiKeyId,
			ProjectName:   resp.Data.ProjectName,
			WalletAddress: resp.Data.WalletAddress,
			ChainId:       resp.Data.ChainId,
			Provider:      resp.Data.Provider,

			FromToken:  resp.Data.FromToken,
			ToToken:    resp.Data.ToToken,
			FromAmount: resp.Data.FromAmount,
			ToAmount:   resp.Data.ToAmount,

			TxFrom:          resp.Data.TxFrom,
			TxTo:            resp.Data.TxTo,
			TxData:          resp.Data.TxData,
			TxValue:         resp.Data.TxValue,
			TxGas:           resp.Data.TxGas,
			TxGasPrice:      resp.Data.TxGasPrice,
			TxNonce:         resp.Data.TxNonce,
			TxSignatureData: resp.Data.TxSignatureData,

			TxHash:        resp.Data.TxHash,
			Status:        resp.Data.Status,
			ErrorCode:     resp.Data.ErrorCode,
			ErrorMessage:  resp.Data.ErrorMessage,
			BroadcastedAt: resp.Data.BroadcastedAt,

			CreatedAt: resp.Data.CreatedAt,
			UpdatedAt: resp.Data.UpdatedAt,

			FromTokenName: resp.Data.FromTokenName,
			ToTokenName:   resp.Data.ToTokenName,
		}
	}
	return out, nil
}
