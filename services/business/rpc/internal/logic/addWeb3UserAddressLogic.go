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

type AddWeb3UserAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddWeb3UserAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddWeb3UserAddressLogic {
	return &AddWeb3UserAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddWeb3UserAddressLogic) AddWeb3UserAddress(in *pb.AddWeb3UserAddressReq) (*pb.AddWeb3UserAddressResp, error) {
	// 参数验证
	deviceID := strings.TrimSpace(in.DeviceId)
	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}

	network := strings.TrimSpace(in.Network)
	if network == "" {
		return nil, errx.Web3NetworkRequired()
	}

	address := strings.TrimSpace(in.Address)
	if address == "" {
		return nil, errx.Web3AddressRequired()
	}
	address = normalizeWeb3Address(in.ChainId, address)

	// 查找 Web3 用户
	web3User, err := l.svcCtx.Web3UserRepository.FindByDeviceID(l.ctx, deviceID)
	if err != nil {
		l.Errorf("查找 Web3 用户失败: device_id=%s, error=%v", deviceID, err)
		return nil, errx.Web3UserNotFound()
	}

	// 检查地址是否已存在（同一用户、同一链、同一地址）
	existingAddr, err := l.svcCtx.Web3UserAddressRepository.FindByUserIDAndChainIDAndAddress(l.ctx, web3User.ID, in.ChainId, address)
	if err == nil && existingAddr != nil {
		// 同一用户已有该地址，直接视为成功
		return &pb.AddWeb3UserAddressResp{
			Success:   true,
			Message:   "ok",
			AddressId: existingAddr.ID,
			Address:   existingAddr.Address,
		}, nil
	}

	// 检查地址是否已被其他用户使用（根据唯一索引 uk_web3_user_addresses_chain_address）
	var existingAddrByChain model.Web3UserAddressModel
	err = l.svcCtx.Web3UserAddressRepository.GetDB().WithContext(l.ctx).
		Where("chain_id = ? AND address = ? AND deleted_at IS NULL", in.ChainId, address).
		First(&existingAddrByChain).Error
	if err == nil {
		// 地址存在但绑定了其他用户，认为是换设备场景，直接转移归属
		now := time.Now()
		updateErr := l.svcCtx.Web3UserAddressRepository.GetDB().WithContext(l.ctx).
			Model(&model.Web3UserAddressModel{}).
			Where("id = ?", existingAddrByChain.ID).
			Updates(map[string]interface{}{
				"web3_user_id": web3User.ID,
				"is_primary":   in.IsPrimary,
				"enabled":      true,
				"updated_at":   now,
				"deleted_at":   gorm.Expr("NULL"),
			}).Error
		if updateErr != nil {
			l.Errorf("转移地址归属失败: chain_id=%d, address=%s, from_user=%d, to_user=%d, error=%v",
				in.ChainId, address, existingAddrByChain.Web3UserID, web3User.ID, updateErr)
			return nil, errx.Web3AddressTransferFailed()
		}

		l.Infof("地址归属已转移（设备更换场景）：chain_id=%d, address=%s, from_user=%d, to_user=%d",
			in.ChainId, address, existingAddrByChain.Web3UserID, web3User.ID)

		l.publishWeb3AddressMonitorUpsert(in.ChainId, address, existingAddrByChain.ID, web3User.ID)
		return &pb.AddWeb3UserAddressResp{
			Success:   true,
			Message:   "ok",
			AddressId: existingAddrByChain.ID,
			Address:   existingAddrByChain.Address,
		}, nil
	} else if err != gorm.ErrRecordNotFound {
		// 数据库查询错误
		l.Errorf("查询地址失败: error=%v", err)
		return nil, errx.Web3AddressQueryFailed()
	}

	// 如果设置为主地址，需要先将该用户的其他主地址取消
	if in.IsPrimary {
		err = l.svcCtx.Web3UserAddressRepository.GetDB().WithContext(l.ctx).
			Model(&model.Web3UserAddressModel{}).
			Where("web3_user_id = ? AND is_primary = 1 AND deleted_at IS NULL", web3User.ID).
			Update("is_primary", false).Error
		if err != nil {
			l.Errorf("取消其他主地址失败: error=%v", err)
			// 继续执行，不阻止创建新地址
		}
	}

	// 创建新地址
	now := time.Now()
	newAddr := &model.Web3UserAddressModel{
		Web3UserID: web3User.ID,
		Network:    network,
		ChainID:    in.ChainId,
		Address:    address,
		IsPrimary:  in.IsPrimary,
		Enabled:    true,
		CreatedAt:  &now,
		UpdatedAt:  &now,
	}

	if err := l.svcCtx.Web3UserAddressRepository.Create(l.ctx, newAddr); err != nil {
		l.Errorf("创建 Web3 地址失败: error=%v", err)
		return nil, errx.Web3AddressCreateFailed()
	}

	l.Infof("成功创建 Web3 地址: user_id=%d, address=%s, chain_id=%d, network=%s",
		web3User.ID, address, in.ChainId, network)

	// 异步从链上同步所有资产余额（主币+代币）到 web3_user_address_balances 表
	go syncWeb3AddressAllAssets(context.Background(), l.svcCtx, newAddr)

	// Publish address monitor event to Kafka (chainsync consumes for near-real-time updates).
	l.publishWeb3AddressMonitorUpsert(in.ChainId, address, newAddr.ID, web3User.ID)

	return &pb.AddWeb3UserAddressResp{
		Success:   true,
		Message:   "ok",
		AddressId: newAddr.ID,
		Address:   newAddr.Address,
	}, nil
}

func (l *AddWeb3UserAddressLogic) publishWeb3AddressMonitorUpsert(chainId int64, address string, addressID int64, userID int64) {
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
func (l *AddWeb3UserAddressLogic) chainIdToBlockChainType(chainId int64) pb.BlockChainType {
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

func chainTypeToMonitorEventChain(chainType pb.BlockChainType) string {
	switch chainType {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM:
		return "ETH"
	case pb.BlockChainType_CHAIN_TYPE_BSC:
		return "BSC"
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		return "TRON"
	default:
		return ""
	}
}
