package logic

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/common/i18n"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"
	"internalwallet/services/chainrpc/rpc/chainrpc"
)

type BuildTronTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBuildTronTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuildTronTransactionLogic {
	return &BuildTronTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BuildTronTransaction 构建 Web3 TRON 交易（返回未签名的 rawData，供客户端签名）
func (l *BuildTronTransactionLogic) BuildTronTransaction(in *pb.BuildTronTransactionReq) (*pb.BuildTronTransactionResp, error) {
	if in == nil {
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: i18n.T(l.ctx, "WEB3_REQUEST_REQUIRED", nil),
		}, nil
	}

	// 验证必填参数
	deviceID := strings.TrimSpace(in.DeviceId)
	fromAddress := strings.TrimSpace(in.FromAddress)
	toAddress := strings.TrimSpace(in.ToAddress)
	amount := strings.TrimSpace(in.Amount)

	if deviceID == "" {
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: i18n.T(l.ctx, "WEB3_DEVICE_ID_REQUIRED", nil),
		}, nil
	}

	if fromAddress == "" {
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: i18n.T(l.ctx, "WEB3_FROM_ADDRESS_REQUIRED", nil),
		}, nil
	}

	if toAddress == "" {
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: i18n.T(l.ctx, "WEB3_TO_ADDRESS_REQUIRED", nil),
		}, nil
	}

	if amount == "" {
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: i18n.T(l.ctx, "WEB3_AMOUNT_REQUIRED", nil),
		}, nil
	}

	// 验证不能转账给自己
	if strings.EqualFold(fromAddress, toAddress) {
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: i18n.T(l.ctx, "WEB3_CANNOT_TRANSFER_TO_SELF", nil),
		}, nil
	}

	// 1. 验证地址是否存在（不验证用户归属，因为地址可以通过助记词导入到不同设备）
	// 注意：余额记录是地址级别的，不绑定到特定用户/设备
	web3UserAddress, err := l.svcCtx.Web3UserAddressRepository.FindByAddress(l.ctx, fromAddress)
	if err != nil || web3UserAddress == nil {
		// 如果地址不存在，仍然允许构建交易（可能是新导入的地址）
		l.Infof("地址未在系统中注册: address=%s (允许继续构建交易)", fromAddress)
	} else {
		// 检查地址是否启用
		if !web3UserAddress.Enabled {
			return &pb.BuildTronTransactionResp{
				Success: false,
				Message: i18n.T(l.ctx, "WEB3_ADDRESS_DISABLED", nil),
			}, nil
		}
	}

	// 2. 检查余额是否足够（对于 TRX 主币转账，需要预留 Gas 费用）
	contractAddress := strings.TrimSpace(in.ContractAddress)
	if contractAddress == "" {
		// TRX 主币转账，需要检查是否有足够的余额支付 Gas
		if err := l.checkTRXBalanceForGas(fromAddress, amount); err != nil {
			return &pb.BuildTronTransactionResp{
				Success: false,
				Message: err.Error(),
			}, nil
		}
	}

	// 3. 调用 ChainRPC 构建 TRON 交易
	if l.svcCtx.ChainRpc == nil {
		l.Errorf("ChainRpc客户端未初始化")
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: i18n.T(l.ctx, "WEB3_CHAIN_SERVICE_UNAVAILABLE", nil),
		}, nil
	}

	// 构建 ChainRPC 请求（使用 chainrpc 包的类型别名）
	buildReq := &chainrpc.ChainRpcBuildTronTransactionReq{
		FromAddress:     fromAddress,
		ToAddress:       toAddress,
		Amount:          amount,
		ContractAddress: contractAddress,
	}

	// 调用 ChainRPC 服务构建交易
	buildResp, err := l.svcCtx.ChainRpc.BuildTronTransaction(l.ctx, buildReq)
	if err != nil {
		l.Errorf("调用 ChainRPC 构建 TRON 交易失败: from_address=%s, to_address=%s, amount=%s, error=%v", fromAddress, toAddress, amount, err)
		// 解析错误消息并国际化
		errorMsg := l.parseAndLocalizeError(err.Error())
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: errorMsg,
		}, nil
	}

	if buildResp == nil || !buildResp.Success {
		errorMsg := i18n.T(l.ctx, "WEB3_BUILD_TX_FAILED", nil)
		if buildResp != nil && buildResp.Message != "" {
			// 解析并国际化 ChainRPC 返回的错误
			errorMsg = l.parseAndLocalizeError(buildResp.Message)
		}
		l.Errorf("ChainRPC 返回失败: %s", errorMsg)
		return &pb.BuildTronTransactionResp{
			Success: false,
			Message: errorMsg,
		}, nil
	}

	return &pb.BuildTronTransactionResp{
		Success: true,
		Message: "OK",
		RawData: buildResp.RawData,
	}, nil
}

// checkTRXBalanceForGas 检查 TRX 余额是否足够支付 Gas 费用
// TRX 转账时，用户需要预留一定的 TRX 作为网络费用
func (l *BuildTronTransactionLogic) checkTRXBalanceForGas(fromAddress string, amountSun string) error {
	// 解析转账金额（单位：SUN，1 TRX = 1,000,000 SUN）
	transferAmount, err := decimal.NewFromString(amountSun)
	if err != nil {
		return &localizedError{message: i18n.T(l.ctx, "WEB3_INVALID_AMOUNT", nil)}
	}

	if transferAmount.LessThanOrEqual(decimal.Zero) {
		return &localizedError{message: i18n.T(l.ctx, "WEB3_AMOUNT_MUST_BE_POSITIVE", nil)}
	}

	// 获取用户 TRX 余额
	var currentBalance decimal.Decimal

	// 优先从数据库查询余额
	if l.svcCtx.Web3UserAddressBalanceRepository != nil {
		balance, err := l.svcCtx.Web3UserAddressBalanceRepository.FindByAddressAndAsset(l.ctx, fromAddress, "TRX", "TRON")
		if err == nil && balance != nil {
			// 解析余额（存储的是原始单位 SUN）
			if parsedBalance, parseErr := decimal.NewFromString(balance.BalanceRaw); parseErr == nil {
				currentBalance = parsedBalance
			}
		}
	}

	// 如果数据库中没有余额记录，尝试从链上查询
	if currentBalance.IsZero() && l.svcCtx.ChainRpc != nil {
		l.Infof("数据库未找到余额记录，从链上查询余额: address=%s", fromAddress)
		balanceResp, err := l.svcCtx.ChainRpc.GetBalance(l.ctx, &chainrpc.GetBalanceReq{
			Address: fromAddress,
			Chain:   pb.ChainRpcType_CHAIN_TYPE_TRON,
		})
		if err == nil && balanceResp != nil && balanceResp.Success {
			if parsedBalance, parseErr := decimal.NewFromString(balanceResp.Balance); parseErr == nil {
				currentBalance = parsedBalance
				l.Infof("从链上查询到余额: address=%s, balance=%s SUN", fromAddress, balanceResp.Balance)
			}
		} else {
			l.Errorf("从链上查询余额失败: address=%s, error=%v", fromAddress, err)
		}
	}

	// 如果仍然无法获取余额，返回错误（不跳过检查）
	if currentBalance.IsZero() {
		l.Errorf("无法获取地址余额，余额检查失败: address=%s", fromAddress)
		return &localizedError{message: i18n.T(l.ctx, "WEB3_INSUFFICIENT_BALANCE_FOR_GAS", nil)}
	}

	// 估算最小 Gas 费用
	// TRX 主币转账通常需要 268 带宽，如果账户有足够的冻结带宽，费用为 0
	// 如果带宽不足，每个带宽单位大约需要 1000 SUN（0.001 TRX），268 带宽 = 268,000 SUN（约 0.268 TRX）
	// 为了安全起见，预留 1 TRX = 1,000,000 SUN 作为网络费用（实际费用通常远低于此）
	minGasReserve := decimal.NewFromInt(1000000) // 1 TRX in SUN

	// 检查余额是否足够
	if currentBalance.LessThan(transferAmount) {
		// 余额小于转账金额，肯定不够
		return &localizedError{message: i18n.T(l.ctx, "WEB3_INSUFFICIENT_BALANCE_FOR_GAS", nil)}
	}

	remainingBalance := currentBalance.Sub(transferAmount)
	if remainingBalance.LessThan(minGasReserve) {
		// 剩余余额不足以支付 Gas
		return &localizedError{message: i18n.T(l.ctx, "WEB3_INSUFFICIENT_BALANCE_FOR_GAS", nil)}
	}

	return nil
}

// localizedError 本地化错误
type localizedError struct {
	message string
}

func (e *localizedError) Error() string {
	return e.message
}

// parseAndLocalizeError 解析并国际化 ChainRPC 返回的错误
func (l *BuildTronTransactionLogic) parseAndLocalizeError(errMsg string) string {
	lowerMsg := strings.ToLower(errMsg)

	// 解析常见的 TRON 错误
	switch {
	case strings.Contains(lowerMsg, "cannot transfer trx to yourself"):
		return i18n.T(l.ctx, "WEB3_CANNOT_TRANSFER_TO_SELF", nil)
	case strings.Contains(lowerMsg, "account resource insufficient"):
		return i18n.T(l.ctx, "WEB3_ACCOUNT_RESOURCE_INSUFFICIENT", nil)
	case strings.Contains(lowerMsg, "account not found") || strings.Contains(lowerMsg, "account not exist"):
		return i18n.T(l.ctx, "WEB3_ACCOUNT_NOT_ACTIVATED", nil)
	case strings.Contains(lowerMsg, "insufficient balance"):
		return i18n.T(l.ctx, "WEB3_INSUFFICIENT_BALANCE_FOR_GAS", nil)
	case strings.Contains(lowerMsg, "bandwidth") || strings.Contains(lowerMsg, "energy"):
		return i18n.T(l.ctx, "WEB3_ACCOUNT_RESOURCE_INSUFFICIENT", nil)
	case strings.Contains(lowerMsg, "failed to get tron client"):
		return i18n.T(l.ctx, "WEB3_TRON_CLIENT_UNAVAILABLE", nil)
	default:
		// 如果是已经格式化的错误消息，尝试提取主要信息
		if strings.Contains(errMsg, "Contract validate error") {
			// 提取 Contract validate error 后面的具体错误
			if idx := strings.Index(errMsg, ":"); idx != -1 {
				specificErr := strings.TrimSpace(errMsg[idx+1:])
				if specificErr != "" {
					return l.parseAndLocalizeError(specificErr)
				}
			}
		}
		// 返回通用构建失败消息
		return i18n.T(l.ctx, "WEB3_BUILD_TX_FAILED", nil)
	}
}
