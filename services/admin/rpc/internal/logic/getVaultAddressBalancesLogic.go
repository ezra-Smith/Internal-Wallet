package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetVaultAddressBalancesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetVaultAddressBalancesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetVaultAddressBalancesLogic {
	return &GetVaultAddressBalancesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetVaultAddressBalancesLogic) GetVaultAddressBalances(in *pb.GetVaultAddressBalancesRequest) (*pb.GetVaultAddressBalancesResponse, error) {

	// 1. 参数验证
	if in.AddressId <= 0 {
		return &pb.GetVaultAddressBalancesResponse{
			Success:   false,
			Message:   "Invalid address ID",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 2. 检查地址是否存在
	addr, err := l.svcCtx.VaultAddressRepo.FindByID(l.ctx, in.AddressId)
	if err != nil {
		return &pb.GetVaultAddressBalancesResponse{
			Success:   false,
			Message:   "Address not found",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, nil
	}

	// 3. 查询网络信息
	network, err := l.svcCtx.VaultNetworkRepo.FindByID(l.ctx, addr.NetworkID)
	if err != nil || network == nil {
		logx.Errorf("Failed to find network for address %d: %v", addr.ID, err)
	}

	networkName := ""
	if network != nil {
		networkName = network.Network
	}

	// 4. 查询该地址的所有余额
	balances, err := l.svcCtx.VaultAddressBalanceRepo.FindByAddressID(l.ctx, in.AddressId)
	if err != nil {
		logx.Errorf("Failed to get balances for address %d: %v", in.AddressId, err)
		return &pb.GetVaultAddressBalancesResponse{
			Success:   false,
			Message:   "Failed to get address balances",
			RequestId: resp.RequestID(l.ctx),
			Timestamp: time.Now().Format(time.RFC3339),
		}, err
	}

	// 5. 检查余额数据是否需要刷新
	// - 数据库中没有余额记录
	// - 或者所有余额记录都超过 5 分钟未同步
	needRefresh := false
	if len(balances) == 0 {
		needRefresh = true
		l.Infof("No balances found for vault address_id=%d, triggering chain sync", in.AddressId)
	} else {
		// 检查是否所有记录都超过 5 分钟未同步
		now := time.Now()
		balanceFreshWindow := 5 * time.Minute
		allStale := true
		for _, b := range balances {
			if b.LastSyncedAt != nil && now.Sub(*b.LastSyncedAt) <= balanceFreshWindow {
				allStale = false
				break
			}
		}
		if allStale {
			needRefresh = true
			l.Infof("All balances are stale for vault address_id=%d, triggering chain sync", in.AddressId)
		}
	}

	// 6. 如果需要刷新，同步链上余额（同步执行，确保返回最新数据）
	if needRefresh {
		syncCtx, syncCancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := SyncVaultAddressBalancesHelper(syncCtx, l.svcCtx, l.Logger, addr); err != nil {
			l.Errorf("Failed to sync vault address balances: address_id=%d err=%v", addr.ID, err)
			// 同步失败不阻塞主流程，继续返回数据库中的旧数据（如果有）
		} else {
			l.Infof("Successfully synced vault address balances: address_id=%d", addr.ID)
			// 重新查询数据库，获取最新余额
			balances, err = l.svcCtx.VaultAddressBalanceRepo.FindByAddressID(syncCtx, in.AddressId)
			if err != nil {
				l.Errorf("Failed to get balances after sync: address_id=%d err=%v", in.AddressId, err)
			}
		}
		syncCancel()
	}

	// 7. 获取资产精度映射（用于格式化余额）
	precisionByAsset := make(map[string]int32)
	assetCodes := make([]string, 0, len(balances))
	for _, b := range balances {
		code := strings.ToUpper(strings.TrimSpace(b.Currency))
		if code != "" {
			assetCodes = append(assetCodes, code)
		}
	}
	if len(assetCodes) > 0 && l.svcCtx.AccountingRpc != nil {
		for _, code := range assetCodes {
			if _, ok := precisionByAsset[code]; ok {
				continue
			}
			accResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: code})
			if err == nil && accResp != nil && accResp.Success && accResp.Item != nil {
				if accResp.Item.Precision >= 0 && accResp.Item.Precision <= 30 {
					precisionByAsset[code] = accResp.Item.Precision
				} else {
					precisionByAsset[code] = 18 // 默认精度
				}
			} else {
				precisionByAsset[code] = 18 // 默认精度
			}
		}
	}

	// 8. 组装余额列表（使用 balance_raw 和精度重新格式化，避免数据库中的 balance 字段错误）
	balanceItems := make([]*pb.VaultAddressBalanceItem, 0, len(balances))
	totalBalanceUSDRaw := int64(0)

	for _, b := range balances {
		lastSyncedAt := ""
		if b.LastSyncedAt != nil {
			lastSyncedAt = b.LastSyncedAt.Format(time.RFC3339)
		}

		// 获取资产精度
		assetCode := strings.ToUpper(strings.TrimSpace(b.Currency))
		precision := precisionByAsset[assetCode]

		// BSC 链上的 USDC/USDT 应该是 18 位精度，而不是 Accounting 服务返回的 6 位
		// 这里根据链和资产类型强制使用正确的精度
		if networkName == "BSC" {
			if assetCode == "USDC" || assetCode == "USDT" {
				precision = 18 // BSC 链上的 USDC/USDT 是 18 位精度
			} else if assetCode == "BNB" {
				precision = 18 // BNB 也是 18 位精度
			}
		} else if networkName == "ETH" || networkName == "Ethereum" {
			if assetCode == "USDC" || assetCode == "USDT" {
				precision = 6 // Ethereum 链上的 USDC/USDT 是 6 位精度
			} else if assetCode == "ETH" {
				precision = 18 // ETH 是 18 位精度
			}
		} else if networkName == "TRON" {
			if assetCode == "USDC" || assetCode == "USDT" {
				precision = 6 // TRON 链上的 USDC/USDT 是 6 位精度
			} else if assetCode == "TRX" {
				precision = 6 // TRX 是 6 位精度
			}
		}

		if precision == 0 {
			precision = 18 // 默认精度
		}

		// 使用 balance_raw 和精度重新格式化余额（避免数据库中的 balance 字段存储错误）
		formattedBalance := rawToFixedAmountString(b.BalanceRaw, precision)
		l.Infof("Formatting balance: currency=%s network=%s balance_raw=%d precision=%d formatted=%s",
			b.Currency, networkName, b.BalanceRaw, precision, formattedBalance)
		balanceItems = append(balanceItems, &pb.VaultAddressBalanceItem{
			Currency:        b.Currency,
			ContractAddress: b.ContractAddress,
			Balance:         formattedBalance,
			BalanceUsd:      centsToUSDString(b.BalanceUSDRaw),
			LastSyncedAt:    lastSyncedAt,
		})

		totalBalanceUSDRaw += b.BalanceUSDRaw
	}

	// 8. 返回结果
	return &pb.GetVaultAddressBalancesResponse{
		Success: true,
		Message: "Success",
		Data: &pb.GetVaultAddressBalancesData{
			AddressId:       addr.ID,
			Address:         addr.Address,
			Network:         networkName,
			Balances:        balanceItems,
			TotalBalanceUsd: centsToUSDString(totalBalanceUSDRaw),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}
