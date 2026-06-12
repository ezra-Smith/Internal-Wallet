package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/common/mq"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type BatchAddWeb3UserAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchAddWeb3UserAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchAddWeb3UserAddressLogic {
	return &BatchAddWeb3UserAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BatchAddWeb3UserAddressLogic) BatchAddWeb3UserAddress(in *pb.BatchAddWeb3UserAddressReq) (*pb.BatchAddWeb3UserAddressResp, error) {
	// 参数验证
	deviceID := strings.TrimSpace(in.DeviceId)
	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}

	if len(in.Addresses) == 0 {
		return nil, errx.Web3AddressListRequired()
	}

	// 限制批量数量（防止一次性添加过多）
	if len(in.Addresses) > 10 {
		return nil, errx.Web3AddressBatchLimitExceeded(10)
	}

	// 查找 Web3 用户
	web3User, err := l.svcCtx.Web3UserRepository.FindByDeviceID(l.ctx, deviceID)
	if err != nil {
		l.Errorf("查找 Web3 用户失败: device_id=%s, error=%v", deviceID, err)
		return nil, errx.Web3UserNotFound()
	}

	// 处理结果
	results := make([]*pb.BatchAddressResult, 0, len(in.Addresses))
	successCount := 0
	failedCount := 0

	// 检查是否有主地址需要设置（批量中只能有一个主地址）
	primaryCount := 0
	var primaryItem *pb.Web3AddressItem
	for _, addrItem := range in.Addresses {
		if addrItem.IsPrimary {
			primaryCount++
			if primaryCount == 1 {
				primaryItem = addrItem
			} else {
				// 如果批量中有多个主地址，只保留第一个，其他的设为 false
				addrItem.IsPrimary = false
				l.Errorf("批量添加中发现多个主地址，只保留第一个: network=%s, chain_id=%d",
					primaryItem.Network, primaryItem.ChainId)
			}
		}
	}

	// 如果批量中有主地址，先取消该用户的其他主地址
	if primaryCount > 0 {
		err = l.svcCtx.Web3UserAddressRepository.GetDB().WithContext(l.ctx).
			Model(&model.Web3UserAddressModel{}).
			Where("web3_user_id = ? AND is_primary = 1 AND deleted_at IS NULL", web3User.ID).
			Update("is_primary", false).Error
		if err != nil {
			l.Errorf("取消其他主地址失败: error=%v", err)
			// 继续执行，不阻止批量创建
		}
	}

	// 批量处理每个地址
	for _, addrItem := range in.Addresses {
		result := l.processAddressItem(web3User.ID, addrItem)
		results = append(results, result)
		if result.Success {
			successCount++
		} else {
			failedCount++
		}
	}

	// 构建响应
	resp := &pb.BatchAddWeb3UserAddressResp{
		TotalCount:   int32(len(in.Addresses)),
		SuccessCount: int32(successCount),
		FailedCount:  int32(failedCount),
		Results:      results,
	}

	if failedCount == 0 {
		resp.Success = true
		resp.Message = "ok"
	} else if successCount == 0 {
		resp.Success = true
		resp.Message = "ok"
	} else {
		resp.Success = true
		resp.Message = "ok"
	}

	l.Infof("批量添加 Web3 地址完成: user_id=%d, total=%d, success=%d, failed=%d",
		web3User.ID, len(in.Addresses), successCount, failedCount)

	// 记录每个地址的处理结果，便于调试
	for _, result := range results {
		if !result.Success {
			l.Errorf("地址添加失败: network=%s, chain_id=%d, address=%s, message=%s",
				result.Network, result.ChainId, result.Address, result.Message)
		} else {
			l.Infof("地址添加成功: network=%s, chain_id=%d, address=%s, address_id=%d",
				result.Network, result.ChainId, result.Address, result.AddressId)
		}
	}

	return resp, nil
}

// processAddressItem 处理单个地址项
func (l *BatchAddWeb3UserAddressLogic) processAddressItem(userID int64, addrItem *pb.Web3AddressItem) *pb.BatchAddressResult {
	result := &pb.BatchAddressResult{
		Network:   addrItem.Network,
		ChainId:   addrItem.ChainId,
		Address:   addrItem.Address,
		Success:   false,
		Message:   "",
		AddressId: 0,
	}

	// 参数验证
	network := strings.TrimSpace(addrItem.Network)
	if network == "" {
		result.Message = "网络名称不能为空"
		return result
	}

	address := strings.TrimSpace(addrItem.Address)
	if address == "" {
		result.Message = "钱包地址不能为空"
		return result
	}
	address = normalizeWeb3Address(addrItem.ChainId, address)
	result.Address = address

	// 检查地址是否已存在（同一用户、同一链、同一地址）
	existingAddr, err := l.svcCtx.Web3UserAddressRepository.FindByUserIDAndChainIDAndAddress(l.ctx, userID, addrItem.ChainId, address)
	if err == nil && existingAddr != nil {
		// 同一用户已有该地址，直接视为成功
		result.Success = true
		result.Message = "ok"
		result.AddressId = existingAddr.ID
		return result
	}

	// 检查地址是否已被其他用户使用（根据唯一索引 uk_web3_user_addresses_chain_address）
	// 注意：唯一索引是 (chain_id, address)，所以同一个地址可以在不同链上使用
	var existingAddrByChain model.Web3UserAddressModel
	err = l.svcCtx.Web3UserAddressRepository.GetDB().WithContext(l.ctx).
		Where("chain_id = ? AND address = ? AND deleted_at IS NULL", addrItem.ChainId, address).
		First(&existingAddrByChain).Error
	if err == nil {
		// 地址存在但绑定了其他用户，认为是同一人换设备，直接转移归属
		now := time.Now()
		updateErr := l.svcCtx.Web3UserAddressRepository.GetDB().WithContext(l.ctx).
			Model(&model.Web3UserAddressModel{}).
			Where("id = ?", existingAddrByChain.ID).
			Updates(map[string]interface{}{
				"web3_user_id": userID,
				"is_primary":   addrItem.IsPrimary,
				"enabled":      true,
				"updated_at":   now,
				"deleted_at":   gorm.Expr("NULL"),
			}).Error
		if updateErr != nil {
			l.Errorf("转移地址归属失败: chain_id=%d, address=%s, from_user=%d, to_user=%d, error=%v",
				addrItem.ChainId, address, existingAddrByChain.Web3UserID, userID, updateErr)
			result.Message = "转移地址失败"
			return result
		}

		l.Infof("地址归属已转移（设备更换场景）：chain_id=%d, address=%s, from_user=%d, to_user=%d",
			addrItem.ChainId, address, existingAddrByChain.Web3UserID, userID)
		result.Success = true
		result.Message = "ok"
		result.AddressId = existingAddrByChain.ID
		l.publishWeb3AddressMonitorUpsert(addrItem.ChainId, address, existingAddrByChain.ID, userID)
		return result
	} else if err != gorm.ErrRecordNotFound {
		// 数据库查询错误
		l.Errorf("查询地址失败: chain_id=%d, address=%s, error=%v", addrItem.ChainId, address, err)
		result.Message = "查询地址失败"
		return result
	}

	// 创建新地址
	now := time.Now()
	newAddr := &model.Web3UserAddressModel{
		Web3UserID: userID,
		Network:    network,
		ChainID:    addrItem.ChainId,
		Address:    address,
		IsPrimary:  addrItem.IsPrimary,
		Enabled:    true,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	if err := l.svcCtx.Web3UserAddressRepository.Create(l.ctx, newAddr); err != nil {
		// 检查是否是唯一索引冲突错误
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			l.Errorf("创建 Web3 地址失败（唯一索引冲突）: chain_id=%d, address=%s, error=%v",
				addrItem.ChainId, address, err)
			result.Message = "该地址已存在或被其他用户使用"
		} else {
			l.Errorf("创建 Web3 地址失败: user_id=%d, chain_id=%d, address=%s, network=%s, error=%v",
				userID, addrItem.ChainId, address, network, err)
			result.Message = "创建地址失败"
		}
		return result
	}

	l.Infof("成功创建 Web3 地址: user_id=%d, address=%s, chain_id=%d, network=%s",
		userID, address, addrItem.ChainId, network)

	// 异步从链上同步所有资产余额（主币+代币）到 web3_user_address_balances 表
	go syncWeb3AddressAllAssets(context.Background(), l.svcCtx, newAddr)

	// Publish address monitor event to Kafka (chainsync consumes for near-real-time updates).
	l.publishWeb3AddressMonitorUpsert(addrItem.ChainId, address, newAddr.ID, userID)

	result.Success = true
	result.Message = "ok"
	result.AddressId = newAddr.ID
	return result
}

func (l *BatchAddWeb3UserAddressLogic) publishWeb3AddressMonitorUpsert(chainId int64, address string, addressID int64, userID int64) {
	if l == nil || l.svcCtx == nil {
		return
	}

	chainType := l.chainIdToBlockChainType(chainId)
	chain := chainTypeToMonitorEventChain(chainType)
	if chain == "" {
		return
	}

	l.svcCtx.PublishAddressMonitorEvent(l.ctx, mq.AddressMonitorEvent{
		Action:  mq.AddressMonitorActionUpsert,
		Source:  mq.AddressMonitorSourceWeb3,
		Chain:   chain,
		Address: address,
		Reason:  "business.web3_user_address.upsert",
		Metadata: map[string]string{
			"web3_user_address_id": fmt.Sprintf("%d", addressID),
			"web3_user_id":         fmt.Sprintf("%d", userID),
			"chain_id":             fmt.Sprintf("%d", chainId),
		},
	})
}

// chainIdToBlockChainType 将 chainId 转换为 BlockChainType
func (l *BatchAddWeb3UserAddressLogic) chainIdToBlockChainType(chainId int64) pb.BlockChainType {
	switch chainId {
	case 1: // Ethereum Mainnet
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	case 11155111: // Sepolia Testnet (Ethereum)
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	case 56: // BSC Mainnet
		return pb.BlockChainType_CHAIN_TYPE_BSC
	case 97: // BSC Testnet
		return pb.BlockChainType_CHAIN_TYPE_BSC
	case 728126428: // TRON (测试网，根据 chainsync.yaml 配置)
		return pb.BlockChainType_CHAIN_TYPE_TRON
	default:
		// 默认返回 UNSPECIFIED，让调用方决定是否跳过
		return pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED
	}
}
