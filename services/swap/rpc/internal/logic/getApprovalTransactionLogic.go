package logic

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetApprovalTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetApprovalTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetApprovalTransactionLogic {
	return &GetApprovalTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetApprovalTransactionLogic) GetApprovalTransaction(in *pb.GetApprovalTransactionRequest) (*pb.GetApprovalTransactionResponse, error) {
	l.Logger.Infow("获取授权交易请求",
		logx.Field("chain_id", in.ChainId),
		logx.Field("wallet_address", in.WalletAddress),
		logx.Field("token_address", in.TokenAddress),
		logx.Field("amount", in.Amount),
	)

	// 1. 参数校验
	if in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid chain_id")
	}

	if in.WalletAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "wallet_address is required")
	}

	if in.TokenAddress == "" {
		return nil, status.Error(codes.InvalidArgument, "token_address is required")
	}

	// 2. 处理 amount 参数
	// "0" -> 重置授权
	// "" 或 "max" -> 无限授权 (2^256 - 1)
	// 其他值 -> 具体授权额度
	amount := in.Amount
	if amount == "" || strings.ToLower(amount) == "max" {
		// 无限授权: 2^256 - 1
		amount = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
		l.Logger.Infow("使用无限授权额度", logx.Field("amount", amount))
	} else if amount == "0" {
		// 重置授权
		l.Logger.Infow("重置授权额度为0")
	} else {
		// 使用指定的授权额度
		l.Logger.Infow("使用指定授权额度", logx.Field("amount", amount))
	}

	// 3. 调用 provider 获取授权交易
	tx, err := l.svcCtx.Provider.GetApprovalTransaction(
		l.ctx,
		in.ChainId,
		in.TokenAddress,
		amount,
		in.WalletAddress,
	)
	if err != nil {
		l.Logger.Errorw("获取授权交易失败",
			logx.Field("error", err),
			logx.Field("chain_id", in.ChainId),
			logx.Field("token_address", in.TokenAddress),
			logx.Field("amount", amount),
		)
		return nil, status.Error(codes.Internal, "failed to get approval transaction: "+err.Error())
	}

	l.Logger.Infow("成功获取授权交易",
		logx.Field("tx_to", tx.To),
		logx.Field("tx_data_length", len(tx.Data)),
	)

	// 4. 构造响应
	return &pb.GetApprovalTransactionResponse{
		Success: true,
		Message: "获取授权交易成功",
		Tx: &pb.UnsignedTransaction{
			ChainId:  tx.ChainID,
			From:     tx.From,
			To:       tx.To,
			Data:     tx.Data,
			Value:    tx.Value,
			Gas:      tx.Gas,
			GasPrice: tx.GasPrice,
		},
	}, nil
}
