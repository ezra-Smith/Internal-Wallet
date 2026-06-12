package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/swap/rpc/internal/interceptor"
	"internalwallet/services/swap/rpc/internal/model"
	"internalwallet/services/swap/rpc/internal/provider"
	"internalwallet/services/swap/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExecuteSwapLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewExecuteSwapLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExecuteSwapLogic {
	return &ExecuteSwapLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ExecuteSwapLogic) ExecuteSwap(in *pb.ExecuteSwapRequest) (*pb.ExecuteSwapResponse, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.ChainId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "chain_id is required")
	}
	if !isValidWalletAddress(in.WalletAddress) {
		return nil, status.Error(codes.InvalidArgument, "invalid wallet_address")
	}
	if !isValidTokenAddress(in.FromTokenAddress) {
		return nil, status.Error(codes.InvalidArgument, "invalid from_token_address")
	}
	if !isValidTokenAddress(in.ToTokenAddress) {
		return nil, status.Error(codes.InvalidArgument, "invalid to_token_address")
	}
	amt, err := parseUintDecimal(in.Amount)
	if err != nil || amt.Sign() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid amount")
	}
	if in.SlippageBps < 0 || in.SlippageBps > 5000 {
		return nil, status.Error(codes.InvalidArgument, "invalid slippage_bps")
	}

	if l.svcCtx.DB == nil || l.svcCtx.TxRepo == nil {
		return nil, status.Error(codes.Internal, "db not configured")
	}
	if l.svcCtx.Provider == nil {
		return nil, status.Error(codes.Internal, "swap provider not ready")
	}

	wallet := normalizeEVMAddress(in.WalletAddress)
	fromToken := strings.TrimSpace(in.FromTokenAddress)
	toToken := strings.TrimSpace(in.ToTokenAddress)
	amount := strings.TrimSpace(in.Amount)

	recipient := strings.TrimSpace(in.Recipient)
	if recipient == "" {
		recipient = wallet
	}
	if !isValidWalletAddress(recipient) {
		return nil, status.Error(codes.InvalidArgument, "invalid recipient")
	}

	slippageBps := in.SlippageBps
	if slippageBps == 0 {
		slippageBps = 50 // default 0.5%
	}

	// 根据 OKX 官方文档推荐的流程：先检查并获取授权交易，再构建 swap 交易
	// 参考: https://web3.okx.com/zh-hans/build/dev-docs/wallet-api/dex-swap-api-introduction
	var approveTx *provider.UnsignedTransaction
	needApprove := in.IncludeApproveTx && !strings.EqualFold(fromToken, nativeTokenAddress)
	if needApprove {
		// 使用无限授权（解决 USDT 等代币的授权问题）
		// USDT 等代币不允许在已有授权的情况下修改授权额度，必须先重置为 0
		// 使用最大值授权可以避免每次都需要重新授权
		// 注意：这是一个较大的授权额度，但这是业界标准做法
		maxApproveAmount := "115792089237316195423570985008687907853269984665640564039457584007913129639935" // 2^256 - 1

		tx, err := l.svcCtx.Provider.GetApprovalTransaction(l.ctx, in.ChainId, fromToken, maxApproveAmount, wallet)
		if err != nil {
			return nil, err
		}
		approveTx = tx
	}

	// 构建 swap 交易（在授权检查之后）
	build, err := l.svcCtx.Provider.BuildSwapTransaction(l.ctx, &provider.SwapRequest{
		ChainID:          in.ChainId,
		WalletAddress:    wallet,
		Recipient:        normalizeEVMAddress(recipient),
		FromTokenAddress: fromToken,
		ToTokenAddress:   toToken,
		Amount:           amount,
		SlippageBps:      slippageBps,
		EstimatedGas:     in.EstimatedGas,
	})
	if err != nil {
		return &pb.ExecuteSwapResponse{
			Success: false,
			Message: err.Error(),
			Data:    nil,
		}, nil
	}

	// Get authenticated context values
	apiKeyID := interceptor.GetAPIKeyID(l.ctx)
	projectName := interceptor.GetProjectName(l.ctx)
	if projectName == "" {
		projectName = "unknown"
	}

	// 序列化 SignatureData（如果存在）
	var signatureDataJSON string
	if len(build.SwapTx.SignatureData) > 0 {
		sigDataBytes, err := json.Marshal(build.SwapTx.SignatureData)
		if err != nil {
			l.Logger.Errorf("failed to marshal signature data", logx.Field("error", err))
		} else {
			signatureDataJSON = string(sigDataBytes)
		}
	}

	record := &model.SwapSvcTransactionModel{
		ApiKeyID:      apiKeyID,
		ProjectName:   projectName,
		WalletAddress: wallet,
		ChainID:       in.ChainId,
		Provider:      strings.TrimSpace(build.Provider),
		FromToken:     normalizeEVMAddress(fromToken),
		ToToken:       normalizeEVMAddress(toToken),
		FromAmount:    amount,
		ToAmount:      "",
		// Save unsigned transaction fields
		TxFrom:          strings.TrimSpace(build.SwapTx.From),
		TxTo:            strings.TrimSpace(build.SwapTx.To),
		TxData:          strings.TrimSpace(build.SwapTx.Data),
		TxValue:         strings.TrimSpace(build.SwapTx.Value),
		TxGas:           strings.TrimSpace(build.SwapTx.Gas),
		TxGasPrice:      strings.TrimSpace(build.SwapTx.GasPrice),
		TxNonce:         "", // 广播时动态查询
		TxSignatureData: signatureDataJSON,
		// Transaction result fields
		TxHash: "",
		Status: "created",
	}
	if build.Quote != nil {
		record.ToAmount = strings.TrimSpace(build.Quote.ToAmount)
	}
	// 如果配置了 TxRepo，持久化交易记录
	if l.svcCtx.TxRepo != nil {
		if err := l.svcCtx.TxRepo.Create(l.ctx, record); err != nil {
			l.Logger.Errorw("create swap transaction record failed", logx.Field("error", err))
			return nil, status.Error(codes.Internal, "failed to persist swap transaction")
		}
	}

	// 使用记录ID
	swapIDStr := fmt.Sprintf("%d", record.ID)
	return &pb.ExecuteSwapResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ExecuteSwapData{
			SwapId:    swapIDStr,
			Provider:  build.Provider,
			Quote:     providerQuoteToPB(build.Quote),
			ApproveTx: providerTxToPB(approveTx),
			SwapTx:    providerTxToPB(&build.SwapTx),
		},
	}, nil
}
