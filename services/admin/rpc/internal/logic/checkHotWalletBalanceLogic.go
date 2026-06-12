package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/common/errcode"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type CheckHotWalletBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckHotWalletBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckHotWalletBalanceLogic {
	return &CheckHotWalletBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckHotWalletBalanceLogic) CheckHotWalletBalance(in *pb.CheckHotWalletBalanceRequest) (*pb.CheckHotWalletBalanceResponse, error) {
	if in == nil || strings.TrimSpace(in.AssetCode) == "" || strings.TrimSpace(in.ChainCode) == "" || strings.TrimSpace(in.Amount) == "" {
		return nil, errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_PARAM", "invalid params", nil)
	}

	assetCode := strings.ToUpper(strings.TrimSpace(in.AssetCode))
	chainCode := strings.ToUpper(strings.TrimSpace(in.ChainCode))
	amount := strings.TrimSpace(in.Amount)

	// 验证金额格式
	amountDec, err := decimal.NewFromString(amount)
	if err != nil || !amountDec.IsPositive() {
		return nil, errcode.New(codes.InvalidArgument, 200, errcode.CodeInvalidParam, "INVALID_AMOUNT", "invalid amount", nil)
	}

	// 1. 通过 SignerRpc 获取热钱包地址
	hotWalletAddr, err := l.getHotWalletAddress(chainCode)
	if err != nil {
		l.Logger.Errorf("获取热钱包地址失败: chain=%s, error=%v", chainCode, err)
		return &pb.CheckHotWalletBalanceResponse{
			Success:   false,
			Code:      int32(errcode.CodeInternalError),
			Message:   "hot wallet not configured",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	// 2. 查询 vault_addresses 表获取 address_id 和 network_id
	vaultAddr, err := l.findVaultAddress(hotWalletAddr, chainCode)
	if err != nil {
		l.Logger.Errorf("查询热钱包 vault address 失败: address=%s, chain=%s, error=%v", hotWalletAddr, chainCode, err)

		// 兜底：尝试自动添加热钱包到 vault_addresses
		l.Logger.Infof("尝试自动添加热钱包到 vault_addresses: chain=%s, address=%s", chainCode, hotWalletAddr)
		if ensureErr := EnsureHotWalletForChain(l.ctx, l.svcCtx, chainCode); ensureErr != nil {
			l.Logger.Errorf("自动添加热钱包失败: %v", ensureErr)
			return &pb.CheckHotWalletBalanceResponse{
				Success:   false,
				Code:      int32(errcode.CodeInternalError),
				Message:   "hot wallet not found in vault and auto-add failed",
				RequestId: resp.RequestID(l.ctx),
				Timestamp: resp.Timestamp(),
			}, nil
		}

		// 重新查询
		vaultAddr, err = l.findVaultAddress(hotWalletAddr, chainCode)
		if err != nil {
			l.Logger.Errorf("自动添加后仍无法查询热钱包: address=%s, chain=%s, error=%v", hotWalletAddr, chainCode, err)
			return &pb.CheckHotWalletBalanceResponse{
				Success:   false,
				Code:      int32(errcode.CodeInternalError),
				Message:   "hot wallet not found",
				RequestId: resp.RequestID(l.ctx),
				Timestamp: resp.Timestamp(),
			}, nil
		}

		l.Logger.Infof("✓ 成功自动添加并查询热钱包: chain=%s, address=%s, vault_id=%d", chainCode, hotWalletAddr, vaultAddr.ID)
	}

	// 2.1 在检查余额前，先同步链上余额确保数据是最新的（避免数据库中的余额数据不准确）
	l.Logger.Infof("同步热钱包链上余额以确保数据最新: address_id=%d, address=%s", vaultAddr.ID, hotWalletAddr)
	vaultAddrModel := &model.VaultAddressModel{
		ID:        vaultAddr.ID,
		NetworkID: vaultAddr.NetworkID,
		Address:   vaultAddr.Address,
	}
	if syncErr := SyncVaultAddressBalancesHelper(l.ctx, l.svcCtx, l.Logger, vaultAddrModel); syncErr != nil {
		l.Logger.Errorf("同步热钱包余额失败（继续使用数据库余额）: address_id=%d, error=%v", vaultAddr.ID, syncErr)
		// 不阻塞主流程，继续使用数据库中的余额进行检查
	} else {
		l.Logger.Infof("✓ 成功同步热钱包余额: address_id=%d", vaultAddr.ID)
	}

	// 3. 获取资产精度（用于格式化显示）
	assetPrecision := int32(18) // 默认
	if l.svcCtx.AccountingRpc != nil {
		if assetResp, assetErr := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode}); assetErr == nil && assetResp != nil && assetResp.Item != nil {
			assetPrecision = assetResp.Item.Precision
		}
	}

	// 4. 检查币种余额和 gas 余额
	nativeToken := l.getNativeTokenForChain(chainCode)
	var assetBalance, gasBalance string
	var sufficient bool = true
	var insufficientReason string

	// 检查资产余额
	if assetCode == nativeToken {
		// 原生币，只需检查一次
		balance, err := l.getBalance(vaultAddr.ID, assetCode, "")
		if err != nil {
			l.Logger.Errorf("查询余额失败: %v", err)
			return &pb.CheckHotWalletBalanceResponse{
				Success:   false,
				Code:      int32(errcode.CodeInternalError),
				Message:   "failed to query balance",
				RequestId: resp.RequestID(l.ctx),
				Timestamp: resp.Timestamp(),
			}, nil
		}
		assetBalance = balance
		gasBalance = balance

		balDec, _ := decimal.NewFromString(balance)
		if balDec.LessThan(amountDec) {
			sufficient = false
			insufficientReason = fmt.Sprintf("热钱包 %s 余额不足（需要: %s, 可用: %s）",
				assetCode, amountDec.StringFixed(int32Min(assetPrecision, 6)), balDec.StringFixed(int32Min(assetPrecision, 6)))
		}
	} else {
		// 代币，需要检查代币余额和 gas 余额
		contractAddr := l.getContractAddress(assetCode, chainCode)
		balance, err := l.getBalance(vaultAddr.ID, assetCode, contractAddr)
		if err != nil {
			l.Logger.Errorf("查询代币余额失败: %v", err)
			return &pb.CheckHotWalletBalanceResponse{
				Success:   false,
				Code:      int32(errcode.CodeInternalError),
				Message:   "failed to query token balance",
				RequestId: resp.RequestID(l.ctx),
				Timestamp: resp.Timestamp(),
			}, nil
		}
		assetBalance = balance

		balDec, _ := decimal.NewFromString(balance)
		if balDec.LessThan(amountDec) {
			sufficient = false
			insufficientReason = fmt.Sprintf("热钱包 %s 余额不足", assetCode)
		}

		// 检查 gas 余额
		gasBalance, err = l.getBalance(vaultAddr.ID, nativeToken, "")
		if err != nil {
			l.Logger.Errorf("查询 gas 余额失败: %v", err)
			return &pb.CheckHotWalletBalanceResponse{
				Success:   false,
				Code:      int32(errcode.CodeInternalError),
				Message:   "failed to query gas balance",
				RequestId: resp.RequestID(l.ctx),
				Timestamp: resp.Timestamp(),
			}, nil
		}

		minGasRequired := l.estimateMinGasRequired(chainCode)
		gasDec, _ := decimal.NewFromString(gasBalance)
		minGasDec, _ := decimal.NewFromString(minGasRequired)
		if gasDec.LessThan(minGasDec) {
			sufficient = false
			if insufficientReason != "" {
				insufficientReason += "; "
			}
			insufficientReason += fmt.Sprintf("热钱包 %s Gas 余额不足", chainCode)
		}
	}

	// 如果余额不足，返回错误
	if !sufficient {
		return &pb.CheckHotWalletBalanceResponse{
			Success: false,
			Code:    int32(errcode.CodeInsufficientBalance),
			Message: insufficientReason,
			Data: &pb.CheckHotWalletBalanceData{
				Sufficient:         false,
				HotWalletAddress:   hotWalletAddr,
				AssetBalance:       assetBalance,
				GasBalance:         gasBalance,
				RequiredAmount:     amountDec.StringFixed(int32Min(assetPrecision, 6)),
				RequiredGas:        l.estimateMinGasRequired(chainCode),
				InsufficientReason: insufficientReason,
			},
			RequestId: resp.RequestID(l.ctx),
			Timestamp: resp.Timestamp(),
		}, nil
	}

	// 余额充足
	return &pb.CheckHotWalletBalanceResponse{
		Success: true,
		Code:    0,
		Message: "sufficient balance",
		Data: &pb.CheckHotWalletBalanceData{
			Sufficient:       true,
			HotWalletAddress: hotWalletAddr,
			AssetBalance:     assetBalance,
			GasBalance:       gasBalance,
			RequiredAmount:   amountDec.StringFixed(int32Min(assetPrecision, 6)),
			RequiredGas:      l.estimateMinGasRequired(chainCode),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

// getHotWalletAddress 获取热钱包地址
func (l *CheckHotWalletBalanceLogic) getHotWalletAddress(chainCode string) (string, error) {
	if l.svcCtx.SignerRpc == nil {
		return "", fmt.Errorf("signer rpc not configured")
	}

	resp, err := l.svcCtx.SignerRpc.GetCompanyWallet(l.ctx, &pb.GetCompanyWalletRequest{
		Chain:       chainCode,
		AddressType: "hot_primary",
		Temperature: 1, // hot wallet
	})
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Code != 0 {
		return "", fmt.Errorf("signer error: code=%d, message=%s", resp.Code, strings.TrimSpace(resp.Message))
	}
	if resp.Wallet == nil || strings.TrimSpace(resp.Wallet.Address) == "" {
		return "", fmt.Errorf("empty company hot wallet address")
	}
	if resp.Wallet.Status != 1 {
		return "", fmt.Errorf("company hot wallet disabled")
	}

	return strings.TrimSpace(resp.Wallet.Address), nil
}

// findVaultAddress 查询 vault_addresses 表
func (l *CheckHotWalletBalanceLogic) findVaultAddress(address, chainCode string) (*struct {
	ID        int64
	NetworkID int64
	Address   string
}, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))

	// 先查询 vault_networks 表获取 network_id（使用 chain_type 或 network 字段匹配）
	chainTypeMap := map[string]string{
		"ETH":      "ethereum",
		"ETHEREUM": "ethereum",
		"BSC":      "bsc",
		"TRON":     "tron",
		"TRX":      "tron",
	}

	chainType := chainTypeMap[chainCode]
	if chainType == "" {
		chainType = strings.ToLower(chainCode)
	}

	var vaultNet struct {
		ID int64 `gorm:"column:id"`
	}

	// 优先使用 chain_type 匹配
	err := l.svcCtx.DB.WithContext(l.ctx).
		Table("vault_networks").
		Where("chain_type = ? AND deleted_at IS NULL", chainType).
		Select("id").
		First(&vaultNet).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 如果 chain_type 找不到，尝试用 network 字段匹配
			err = l.svcCtx.DB.WithContext(l.ctx).
				Table("vault_networks").
				Where("UPPER(network) = ? AND deleted_at IS NULL", chainCode).
				Select("id").
				First(&vaultNet).Error

			if err != nil {
				return nil, fmt.Errorf("network not found: %s (tried chain_type=%s and network=%s)", chainCode, chainType, chainCode)
			}
		} else {
			return nil, fmt.Errorf("failed to query vault_network: %w", err)
		}
	}

	// 查询 vault_addresses
	vaultAddrRepo := repository.NewVaultAddressRepository(l.svcCtx.DB)
	vaultAddr, err := vaultAddrRepo.FindByAddress(l.ctx, vaultNet.ID, address)
	if err != nil || vaultAddr == nil {
		return nil, fmt.Errorf("vault address not found")
	}

	return &struct {
		ID        int64
		NetworkID int64
		Address   string
	}{
		ID:        vaultAddr.ID,
		NetworkID: vaultAddr.NetworkID,
		Address:   vaultAddr.Address,
	}, nil
}

// getBalance 查询余额（使用 balance_raw 和精度重新计算，避免数据库中的 balance 字段存储错误）
func (l *CheckHotWalletBalanceLogic) getBalance(addressID int64, currency, contractAddress string) (string, error) {
	balanceRepo := repository.NewVaultAddressBalanceRepository(l.svcCtx.DB)
	balanceRecord, err := balanceRepo.FindByAddressAndCurrency(l.ctx, addressID, currency, contractAddress)
	if err != nil || balanceRecord == nil {
		return "0", nil // 没有记录返回 0
	}

	// 如果 balance_raw 为 0，直接返回 "0"
	if balanceRecord.BalanceRaw == 0 {
		return "0", nil
	}

	// 获取网络信息以确定链类型
	var networkName string
	if l.svcCtx.VaultNetworkRepo != nil {
		if net, netErr := l.svcCtx.VaultNetworkRepo.FindByID(l.ctx, balanceRecord.NetworkID); netErr == nil && net != nil {
			networkName = net.Network
		}
	}

	// 获取资产精度
	assetCode := strings.ToUpper(strings.TrimSpace(currency))
	precision := int32(18) // 默认精度
	if l.svcCtx.AccountingRpc != nil {
		if assetResp, assetErr := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode}); assetErr == nil && assetResp != nil && assetResp.Item != nil {
			precision = assetResp.Item.Precision
		}
	}

	// 根据网络和资产类型强制使用正确的精度（与 getVaultAddressBalancesLogic 保持一致）
	if networkName == "BSC" {
		if assetCode == "USDC" || assetCode == "USDT" {
			precision = 18 // BSC 链上的 USDC/USDT 是 18 位精度
		} else if assetCode == "BNB" {
			precision = 18
		}
	} else if networkName == "ETH" || networkName == "Ethereum" {
		if assetCode == "USDC" || assetCode == "USDT" {
			precision = 6 // Ethereum 链上的 USDC/USDT 是 6 位精度
		} else if assetCode == "ETH" {
			precision = 18
		}
	} else if networkName == "TRON" {
		if assetCode == "USDC" || assetCode == "USDT" {
			precision = 6 // TRON 链上的 USDC/USDT 是 6 位精度
		} else if assetCode == "TRX" {
			precision = 6
		}
	}

	// 使用 balance_raw 和精度重新格式化余额（避免数据库中的 balance 字段存储错误）
	formattedBalance := rawToFixedAmountString(balanceRecord.BalanceRaw, precision)
	l.Logger.Infof("getBalance: currency=%s network=%s balance_raw=%d precision=%d formatted=%s",
		currency, networkName, balanceRecord.BalanceRaw, precision, formattedBalance)
	return formattedBalance, nil
}

// getContractAddress 获取合约地址
func (l *CheckHotWalletBalanceLogic) getContractAddress(assetCode, chainCode string) string {
	if l.svcCtx.CurrencyChainSettingsRepo == nil {
		return ""
	}

	setting, err := l.svcCtx.CurrencyChainSettingsRepo.FindByAssetAndChain(l.ctx, assetCode, chainCode)
	if err != nil || setting == nil || setting.ContractAddress == nil {
		return ""
	}

	return strings.TrimSpace(*setting.ContractAddress)
}

// getNativeTokenForChain 获取链的原生代币
func (l *CheckHotWalletBalanceLogic) getNativeTokenForChain(chainCode string) string {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "TRON":
		return "TRX"
	case "ETH", "ETHEREUM":
		return "ETH"
	case "BSC", "BNB":
		return "BNB"
	default:
		return chainCode // 默认返回链名称本身
	}
}

// estimateMinGasRequired 估算最低 gas 需求（简化版本）
func (l *CheckHotWalletBalanceLogic) estimateMinGasRequired(chainCode string) string {
	switch strings.ToUpper(strings.TrimSpace(chainCode)) {
	case "TRON":
		return "10" // 10 TRX（保守估计，用于能量和带宽。实际 TRC20 转账通常只需要 1-5 TRX）
	case "ETH", "ETHEREUM":
		return "0.01" // 0.01 ETH
	case "BSC", "BNB":
		return "0.005" // 0.005 BNB
	default:
		return "0.01"
	}
}

// int32Min 返回两个 int32 的最小值
func int32Min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
