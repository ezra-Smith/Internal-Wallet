package logic

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"
	"internalwallet/services/swap/rpc/swap"
)

type SwapGetApprovalTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapGetApprovalTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapGetApprovalTransactionLogic {
	return &SwapGetApprovalTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SwapGetApprovalTransaction 获取授权交易（代理到 Swap 服务）
func (l *SwapGetApprovalTransactionLogic) SwapGetApprovalTransaction(in *pb.BusinessSwapGetApprovalTransactionRequest) (*pb.BusinessSwapGetApprovalTransactionResponse, error) {
	l.Logger.Infow("代理获取授权交易请求",
		logx.Field("chain_id", in.ChainId),
		logx.Field("wallet_address", in.WalletAddress),
		logx.Field("token_address", in.TokenAddress),
		logx.Field("amount", in.Amount),
	)

	// 参数校验
	if in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid chain_id")
	}

	if in.WalletAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "wallet_address is required")
	}

	if in.TokenAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "token_address is required")
	}

	// 调用 Swap 服务
	resp, err := l.svcCtx.SwapRpc.GetApprovalTransaction(l.ctx, &swap.GetApprovalTransactionRequest{
		ChainId:       in.ChainId,
		WalletAddress: in.WalletAddress,
		TokenAddress:  in.TokenAddress,
		Amount:        in.Amount,
	})
	if err != nil {
		l.Logger.Errorw("调用 Swap 服务获取授权交易失败",
			logx.Field("error", err),
		)
		return nil, err
	}

	// 构造响应
	out := &pb.BusinessSwapGetApprovalTransactionResponse{
		Success: resp.Success,
		Message: resp.Message,
	}

	if resp.Tx != nil {
		out.Tx = &pb.BusinessSwapUnsignedTransaction{
			ChainId:  resp.Tx.ChainId,
			From:     resp.Tx.From,
			To:       resp.Tx.To,
			Data:     resp.Tx.Data,
			Value:    resp.Tx.Value,
			Gas:      resp.Tx.Gas,
			GasPrice: resp.Tx.GasPrice,
		}
	}

	l.Logger.Infow("成功代理获取授权交易",
		logx.Field("success", out.Success),
	)

	return out, nil
}
