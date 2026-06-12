package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/common/errcode"
	"internalwallet/common/i18n"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

// HotWalletBalanceChecker 检查热钱包余额是否足够支付提现
type HotWalletBalanceChecker struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logger logx.Logger
}

// NewHotWalletBalanceChecker 创建热钱包余额检查器
func NewHotWalletBalanceChecker(ctx context.Context, svcCtx *svc.ServiceContext) *HotWalletBalanceChecker {
	return &HotWalletBalanceChecker{
		ctx:    ctx,
		svcCtx: svcCtx,
		logger: logx.WithContext(ctx),
	}
}

// CheckHotWalletBalance 检查热钱包余额和 gas 是否足够
// assetCode: 提现币种
// chainCode: 提现链
// amount: 提现金额（decimal string）
// 返回错误如果余额不足
func (c *HotWalletBalanceChecker) CheckHotWalletBalance(assetCode, chainCode string, amount string) error {
	if c.svcCtx == nil || c.svcCtx.AdminRpc == nil {
		return fmt.Errorf("admin rpc not configured")
	}

	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))

	// 调用 admin-rpc 的 CheckHotWalletBalance 方法
	resp, err := c.svcCtx.AdminRpc.CheckHotWalletBalance(c.ctx, &pb.CheckHotWalletBalanceRequest{
		AssetCode: assetCode,
		ChainCode: chainCode,
		Amount:    amount,
	})

	if err != nil {
		c.logger.Errorf("调用 admin-rpc CheckHotWalletBalance 失败: asset=%s, chain=%s, error=%v", assetCode, chainCode, err)
		return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "HOT_WALLET_CHECK_FAILED",
			"hot wallet balance check failed", nil)
	}

	// 检查响应
	if resp == nil {
		return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "HOT_WALLET_CHECK_FAILED",
			"empty response from admin rpc", nil)
	}

	// 如果余额不足，返回错误
	if !resp.Success || resp.Code != 0 {
		if resp.Data != nil && !resp.Data.Sufficient {
			// 不暴露具体的热钱包余额不足信息，统一返回账号状态异常
			// 详细原因记录在日志中，不返回给用户
			detailedReason := strings.TrimSpace(resp.Data.InsufficientReason)
			if detailedReason == "" {
				detailedReason = strings.TrimSpace(resp.Message)
			}
			if detailedReason == "" {
				detailedReason = fmt.Sprintf("热钱包 %s 余额不足", assetCode)
			}
			c.logger.Errorf("热钱包余额不足（不返回给用户）: asset=%s, chain=%s, reason=%s", assetCode, chainCode, detailedReason)
			
			// 使用 i18n 翻译错误消息
			localizedMsg := i18n.T(c.ctx, "ACCOUNT_STATUS_ABNORMAL", nil)
			if localizedMsg == "" || localizedMsg == "ACCOUNT_STATUS_ABNORMAL" {
				// 如果翻译失败，使用默认消息
				localizedMsg = "账号状态异常，请联系客服"
			}
			
			// 返回通用的账号状态异常错误，通过 error_detail 传递国际化消息
			return errcode.NewWithMetadata(
				codes.FailedPrecondition,
				200,
				errcode.CodeInsufficientBalance,
				"ACCOUNT_STATUS_ABNORMAL",
				"account status abnormal",  // gRPC desc，会出现在日志中
				nil,
				map[string]string{"error_detail": localizedMsg}, // API Gateway 会优先使用这个
			)
		}
		// 其他错误
		return errcode.New(codes.Internal, 500, errcode.CodeInternalError, "HOT_WALLET_CHECK_FAILED",
			strings.TrimSpace(resp.Message), nil)
	}

	return nil
}

