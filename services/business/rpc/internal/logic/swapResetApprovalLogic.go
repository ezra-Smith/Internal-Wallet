package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwapResetApprovalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwapResetApprovalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwapResetApprovalLogic {
	return &SwapResetApprovalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SwapResetApproval 重置代币授权并广播
// 用于解决 USDT 等代币的授权问题
// 这些代币不允许在已有授权的情况下修改授权额度，必须先重置为 0
//
// 使用方式：
// 1. 前端构造重置授权交易并签名
// 2. 前端调用此接口传入 signed_tx_hex
// 3. 后端广播交易到链上
func (l *SwapResetApprovalLogic) SwapResetApproval(in *pb.BusinessSwapResetApprovalRequest) (*pb.BusinessSwapResetApprovalResponse, error) {
	// 参数验证
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
	if strings.TrimSpace(in.TokenAddress) == "" {
		return nil, errx.InvalidTokenAddress()
	}
	if strings.TrimSpace(in.SignedTxHex) == "" {
		return nil, errx.InvalidParam("signed_tx_hex is required")
	}

	// 调用 swap 服务
	resp, err := l.svcCtx.SwapRpc.ResetApproval(l.ctx, &pb.ResetApprovalRequest{
		ChainId:       in.ChainId,
		WalletAddress: in.WalletAddress,
		TokenAddress:  in.TokenAddress,
		SignedTxHex:   in.SignedTxHex, // 传递签名后的交易（如果有）
	})
	if err != nil {
		l.Errorf("reset approval failed: %v", err)
		return nil, errx.SwapServiceError()
	}
	if resp == nil {
		return nil, errx.SwapServiceError()
	}
	if !resp.Success {
		return nil, errx.Internal(resp.Message)
	}

	// 转换响应
	out := &pb.BusinessSwapResetApprovalResponse{
		Success: true,
		Message: resp.Message,
	}
	if resp.Data != nil {
		out.Data = &pb.BusinessSwapResetApprovalData{
			TxHash:        resp.Data.TxHash,
			BroadcastedAt: resp.Data.BroadcastedAt,
		}
	}

	return out, nil
}
