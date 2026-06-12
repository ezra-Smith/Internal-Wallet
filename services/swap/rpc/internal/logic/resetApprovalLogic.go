package logic

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/svc"
)

type ResetApprovalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResetApprovalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetApprovalLogic {
	return &ResetApprovalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResetApproval 广播重置授权交易
// 用于解决 USDT 等代币的授权问题：这些代币不允许在已有授权的情况下修改授权额度
// 必须先将授权重置为 0，然后才能设置新的授权额度
//
// 使用流程：
// 1. 前端调用 GetApprovalTransaction 接口获取未签名的重置授权交易（amount="0"）
// 2. 前端在客户端签名交易
// 3. 前端调用本接口，传入签名后的交易 hex
// 4. 后端验证基本信息并广播交易到区块链
func (l *ResetApprovalLogic) ResetApproval(in *pb.ResetApprovalRequest) (*pb.ResetApprovalResponse, error) {
	l.Logger.Infow("重置授权广播请求",
		logx.Field("chain_id", in.ChainId),
		logx.Field("wallet_address", in.WalletAddress),
		logx.Field("token_address", in.TokenAddress),
	)

	// 参数验证
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "chain_id is required")
	}
	if !isValidWalletAddress(in.WalletAddress) {
		return nil, status.Error(codes.InvalidArgument, "invalid wallet_address")
	}
	if !isValidTokenAddress(in.TokenAddress) {
		return nil, status.Error(codes.InvalidArgument, "invalid token_address")
	}
	if strings.TrimSpace(in.SignedTxHex) == "" {
		return nil, status.Error(codes.InvalidArgument, "signed_tx_hex is required")
	}

	tokenAddress := strings.TrimSpace(in.TokenAddress)

	// 检查是否为原生代币（原生代币不需要授权）
	if strings.EqualFold(tokenAddress, nativeTokenAddress) {
		return nil, status.Error(codes.InvalidArgument, "native token does not require approval")
	}

	// 解析签名交易
	signedTxHex := strings.TrimSpace(in.SignedTxHex)
	signedTxHex = strings.TrimPrefix(signedTxHex, "0x")

	txBytes, err := hex.DecodeString(signedTxHex)
	if err != nil {
		l.Logger.Errorw("解析签名交易失败",
			logx.Field("error", err),
			logx.Field("signed_tx_hex", signedTxHex[:20]+"..."),
		)
		return nil, status.Error(codes.InvalidArgument, "invalid signed transaction hex")
	}

	signedTx := new(types.Transaction)
	if err := signedTx.UnmarshalBinary(txBytes); err != nil {
		l.Logger.Errorw("反序列化签名交易失败",
			logx.Field("error", err),
		)
		return nil, status.Error(codes.InvalidArgument, "failed to unmarshal signed transaction")
	}

	// 基本验证：恢复签名者地址
	signer := types.LatestSignerForChainID(signedTx.ChainId())
	from, err := signer.Sender(signedTx)
	if err != nil {
		l.Logger.Errorw("获取签名者地址失败",
			logx.Field("error", err),
		)
		return nil, status.Error(codes.InvalidArgument, "failed to recover signer from transaction")
	}

	if !strings.EqualFold(from.Hex(), in.WalletAddress) {
		l.Logger.Errorw("交易发送方与钱包地址不匹配",
			logx.Field("tx_from", from.Hex()),
			logx.Field("wallet_address", in.WalletAddress),
		)
		return nil, status.Error(codes.InvalidArgument, "transaction sender does not match wallet address")
	}

	// 验证 to 地址是代币合约
	if signedTx.To() == nil {
		l.Logger.Errorw("交易缺少 to 地址")
		return nil, status.Error(codes.InvalidArgument, "transaction missing to address")
	}

	if !strings.EqualFold(signedTx.To().Hex(), in.TokenAddress) {
		l.Logger.Errorw("交易 to 地址与代币合约地址不匹配",
			logx.Field("tx_to", signedTx.To().Hex()),
			logx.Field("token_address", in.TokenAddress),
		)
		return nil, status.Error(codes.InvalidArgument, "transaction to address does not match token contract")
	}

	l.Logger.Infow("签名交易基本验证通过",
		logx.Field("from", from.Hex()),
		logx.Field("to", signedTx.To().Hex()),
		logx.Field("chain_id", signedTx.ChainId()),
	)

	// 广播交易
	ethClient, err := l.svcCtx.EVM.GetClient(l.ctx, in.ChainId)
	if err != nil {
		l.Logger.Errorw("获取 EVM 客户端失败",
			logx.Field("error", err),
			logx.Field("chain_id", in.ChainId),
		)
		return nil, status.Error(codes.Internal, "failed to get EVM client")
	}

	if err := ethClient.SendTransaction(context.Background(), signedTx); err != nil {
		l.Logger.Errorw("广播交易失败",
			logx.Field("error", err),
			logx.Field("tx_hash", signedTx.Hash().Hex()),
		)
		return nil, status.Error(codes.Internal, "failed to broadcast transaction: "+err.Error())
	}

	txHash := signedTx.Hash().Hex()
	broadcastedAt := time.Now().Unix()

	l.Logger.Infow("重置授权交易已成功广播",
		logx.Field("tx_hash", txHash),
		logx.Field("broadcasted_at", broadcastedAt),
	)

	return &pb.ResetApprovalResponse{
		Success: true,
		Message: "reset approval transaction broadcasted successfully",
		Data: &pb.ResetApprovalData{
			TxHash:        txHash,
			BroadcastedAt: broadcastedAt,
		},
	}, nil
}
