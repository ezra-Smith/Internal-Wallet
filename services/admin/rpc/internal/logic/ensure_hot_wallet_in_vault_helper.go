package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// EnsureCompanyHotWalletsInVault 确保 company_wallets 中的所有热钱包地址都在 vault_addresses 表中
// 这个函数应该在同步余额前调用，确保热钱包地址已注册
func EnsureCompanyHotWalletsInVault(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger) error {
	if svcCtx == nil || svcCtx.SignerRpc == nil || svcCtx.DB == nil {
		return fmt.Errorf("service context not properly configured")
	}

	// 1. 从 Signer 服务获取所有活跃的热钱包
	hotWallets, err := getAllCompanyHotWallets(ctx, svcCtx)
	if err != nil {
		logger.Errorf("获取公司热钱包列表失败: %v", err)
		return err
	}

	if len(hotWallets) == 0 {
		logger.Info("没有找到公司热钱包，跳过同步")
		return nil
	}

	// 2. 获取 vault_networks 信息
	vaultNetRepo := repository.NewVaultNetworkRepository(svcCtx.DB)
	networks, err := vaultNetRepo.List(ctx, "")
	if err != nil {
		logger.Errorf("查询 vault_networks 失败: %v", err)
		return err
	}

	// 创建映射表：使用 chain_type 作为 key（更可靠）
	networkByChainType := make(map[string]*model.VaultNetworkModel)
	networkByNetwork := make(map[string]*model.VaultNetworkModel)
	for _, net := range networks {
		if net != nil {
			// 按 chain_type 建立索引（例如："ethereum", "bsc", "tron"）
			chainType := strings.ToLower(strings.TrimSpace(net.ChainType))
			if chainType != "" {
				networkByChainType[chainType] = net
			}
			// 按 network 字段建立索引（例如："Ethereum", "BSC", "Tron"）
			networkName := strings.ToUpper(strings.TrimSpace(net.Network))
			if networkName != "" {
				networkByNetwork[networkName] = net
			}
		}
	}

	// 3. 检查每个热钱包是否在 vault_addresses 中，如果不在则添加
	vaultAddrRepo := repository.NewVaultAddressRepository(svcCtx.DB)
	addedCount := 0
	skippedCount := 0

	for _, wallet := range hotWallets {
		chain := strings.ToUpper(strings.TrimSpace(wallet.Chain))
		
		// 将 company_wallets.chain 映射到 vault_networks（优先使用 chain_type 匹配）
		network := findVaultNetworkForChain(chain, networkByChainType, networkByNetwork)
		if network == nil {
			logger.Errorf("热钱包链 %s 在 vault_networks 中不存在，跳过: address=%s", chain, wallet.Address)
			skippedCount++
			continue
		}
		
		logger.Infof("找到 vault_network 映射: company_wallet.chain=%s -> vault_network(id=%d, network=%s, chain_type=%s)",
			chain, network.ID, network.Network, network.ChainType)

		// 检查地址是否已存在
		existing, err := vaultAddrRepo.FindByAddress(ctx, network.ID, wallet.Address)
		if err != nil && err.Error() != "address not found" {
			logger.Errorf("查询 vault_address 失败: chain=%s, address=%s, err=%v", chain, wallet.Address, err)
			continue
		}

		if existing != nil {
			logger.Infof("热钱包地址已存在于 vault_addresses: chain=%s, address=%s, vault_id=%d", 
				chain, wallet.Address, existing.ID)
			continue
		}

		// 地址不存在，需要添加
		logger.Infof("正在添加热钱包到 vault_addresses: chain=%s, address_type=%s, address=%s",
			chain, wallet.AddressType, wallet.Address)

		label := fmt.Sprintf("系统热钱包 - %s - %s", chain, wallet.AddressType)
		newAddr := &model.VaultAddressModel{
			NetworkID:      network.ID,
			Address:        wallet.Address,
			AddressType:    mapAddressType(wallet.AddressType),
			Label:          &label,
			Status:         "active",
			IsActiveWallet: getIsActiveWallet(wallet.AddressType),
			CreatedBy:      0, // 系统自动创建
		}

		if err := vaultAddrRepo.Create(ctx, newAddr); err != nil {
			logger.Errorf("添加热钱包到 vault_addresses 失败: chain=%s, address=%s, err=%v",
				chain, wallet.Address, err)
			continue
		}

		logger.Infof("✓ 成功添加热钱包到 vault_addresses: chain=%s, address=%s, vault_id=%d",
			chain, wallet.Address, newAddr.ID)
		addedCount++
	}

	if addedCount > 0 {
		logger.Infof("热钱包同步完成: 新增=%d, 跳过=%d, 总数=%d", addedCount, skippedCount, len(hotWallets))
	} else {
		logger.Infof("热钱包同步完成: 所有热钱包已存在，无需添加")
	}

	return nil
}

// CompanyHotWallet 表示公司热钱包信息
type CompanyHotWallet struct {
	Chain       string
	Address     string
	AddressType string
	Temperature int32
	Status      int32
}

// getAllCompanyHotWallets 从 Signer 服务获取所有活跃的热钱包（hot_primary 和 hot_backup）
func getAllCompanyHotWallets(ctx context.Context, svcCtx *svc.ServiceContext) ([]CompanyHotWallet, error) {
	// 支持的链
	chains := []string{"TRON", "ETH", "BSC"}
	// 热钱包类型
	addressTypes := []string{"hot_primary", "hot_backup"}

	var hotWallets []CompanyHotWallet

	for _, chain := range chains {
		for _, addrType := range addressTypes {
			resp, err := svcCtx.SignerRpc.GetCompanyWallet(ctx, &pb.GetCompanyWalletRequest{
				Chain:       chain,
				AddressType: addrType,
				Temperature: 1, // hot
			})

			if err != nil {
				logx.Errorf("查询热钱包失败: chain=%s, address_type=%s, err=%v", chain, addrType, err)
				continue
			}

			if resp == nil || resp.Code != 0 || resp.Wallet == nil {
				logx.Infof("未找到热钱包: chain=%s, address_type=%s", chain, addrType)
				continue
			}

			wallet := resp.Wallet
			if wallet.Status != 1 {
				logx.Infof("热钱包已禁用: chain=%s, address_type=%s, address=%s",
					chain, addrType, wallet.Address)
				continue
			}

			if strings.TrimSpace(wallet.Address) == "" {
				logx.Errorf("热钱包地址为空: chain=%s, address_type=%s", chain, addrType)
				continue
			}

			hotWallets = append(hotWallets, CompanyHotWallet{
				Chain:       chain,
				Address:     strings.TrimSpace(wallet.Address),
				AddressType: addrType,
				Temperature: wallet.Temperature,
				Status:      wallet.Status,
			})

			logx.Infof("找到热钱包: chain=%s, address_type=%s, address=%s",
				chain, addrType, wallet.Address)
		}
	}

	return hotWallets, nil
}

// mapAddressType 映射 company_wallets 的 address_type 到 vault_addresses 的 address_type
func mapAddressType(companyType string) string {
	switch strings.ToLower(strings.TrimSpace(companyType)) {
	case "hot_primary":
		return "hot"
	case "hot_backup":
		return "hot"
	case "cold_primary":
		return "cold"
	case "cold_backup":
		return "cold"
	default:
		return "active"
	}
}

// getIsActiveWallet 判断是否为活跃钱包（只有 hot_primary 设置为 1）
func getIsActiveWallet(addressType string) int8 {
	if strings.EqualFold(strings.TrimSpace(addressType), "hot_primary") {
		return 1
	}
	return 0
}

// findVaultNetworkForChain 根据 company_wallets.chain 查找对应的 vault_network
// 参数 chain: 来自 company_wallets 表的链名称（例如："ETH", "BSC", "TRON"）
// 返回: 对应的 VaultNetworkModel，如果找不到则返回 nil
func findVaultNetworkForChain(chain string, networkByChainType, networkByNetwork map[string]*model.VaultNetworkModel) *model.VaultNetworkModel {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	
	// 1. 映射 company_wallets.chain 到标准的 chain_type
	chainTypeMap := map[string]string{
		"ETH":     "ethereum",
		"ETHEREUM": "ethereum",
		"BSC":     "bsc",
		"TRON":    "tron",
		"TRX":     "tron",
	}
	
	chainType := chainTypeMap[chain]
	if chainType != "" {
		if net, exists := networkByChainType[chainType]; exists {
			return net
		}
	}
	
	// 2. 尝试直接用 network 字段匹配（兼容不同的命名）
	networkNameMap := map[string][]string{
		"ETH": {"ETHEREUM", "ETH"},
		"BSC": {"BSC"},
		"TRON": {"TRON", "TRX"},
	}
	
	possibleNames := networkNameMap[chain]
	for _, name := range possibleNames {
		if net, exists := networkByNetwork[name]; exists {
			return net
		}
	}
	
	// 3. 最后尝试直接匹配（大小写不敏感）
	if net, exists := networkByNetwork[chain]; exists {
		return net
	}
	
	return nil
}

// EnsureHotWalletForChain 确保指定链的热钱包在 vault_addresses 中
// 这个函数可以在提现检查前调用，作为兜底
func EnsureHotWalletForChain(ctx context.Context, svcCtx *svc.ServiceContext, chainCode string) error {
	if svcCtx == nil || svcCtx.SignerRpc == nil || svcCtx.DB == nil {
		return fmt.Errorf("service context not configured")
	}

	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))

	// 1. 从 Signer 获取热钱包地址
	resp, err := svcCtx.SignerRpc.GetCompanyWallet(ctx, &pb.GetCompanyWalletRequest{
		Chain:       chainCode,
		AddressType: "hot_primary",
		Temperature: 1,
	})

	if err != nil {
		return fmt.Errorf("failed to get hot wallet from signer: %w", err)
	}

	if resp == nil || resp.Code != 0 || resp.Wallet == nil {
		return fmt.Errorf("hot wallet not configured in company_wallets: chain=%s", chainCode)
	}

	hotWalletAddr := strings.TrimSpace(resp.Wallet.Address)
	if hotWalletAddr == "" {
		return fmt.Errorf("hot wallet address is empty")
	}

	// 2. 查询 vault_networks（使用 chain_type 或 network 字段匹配）
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
		ID        int64  `gorm:"column:id"`
		Network   string `gorm:"column:network"`
		ChainType string `gorm:"column:chain_type"`
	}
	
	// 优先使用 chain_type 匹配
	err = svcCtx.DB.WithContext(ctx).
		Table("vault_networks").
		Where("chain_type = ? AND deleted_at IS NULL", chainType).
		Select("id, network, chain_type").
		First(&vaultNet).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 如果 chain_type 找不到，尝试用 network 字段匹配
			err = svcCtx.DB.WithContext(ctx).
				Table("vault_networks").
				Where("UPPER(network) = ? AND deleted_at IS NULL", chainCode).
				Select("id, network, chain_type").
				First(&vaultNet).Error
			
			if err != nil {
				return fmt.Errorf("vault_network not found for chain: %s (tried chain_type=%s and network=%s)", 
					chainCode, chainType, chainCode)
			}
		} else {
			return fmt.Errorf("failed to query vault_network: %w", err)
		}
	}
	
	logx.Infof("找到 vault_network: chain=%s -> vault_network(id=%d, network=%s, chain_type=%s)",
		chainCode, vaultNet.ID, vaultNet.Network, vaultNet.ChainType)

	// 3. 检查是否已存在
	vaultAddrRepo := repository.NewVaultAddressRepository(svcCtx.DB)
	existing, err := vaultAddrRepo.FindByAddress(ctx, vaultNet.ID, hotWalletAddr)
	if err != nil && err.Error() != "address not found" {
		return fmt.Errorf("failed to check vault_address: %w", err)
	}

	if existing != nil {
		// 已存在，无需添加
		return nil
	}

	// 4. 添加到 vault_addresses
	label := fmt.Sprintf("系统热钱包 - %s - hot_primary", chainCode)
	newAddr := &model.VaultAddressModel{
		NetworkID:      vaultNet.ID,
		Address:        hotWalletAddr,
		AddressType:    "hot",
		Label:          &label,
		Status:         "active",
		IsActiveWallet: 1,
		CreatedBy:      0,
	}

	if err := vaultAddrRepo.Create(ctx, newAddr); err != nil {
		return fmt.Errorf("failed to add hot wallet to vault_addresses: %w", err)
	}

	logx.Infof("自动添加热钱包到 vault_addresses: chain=%s, address=%s, vault_id=%d",
		chainCode, hotWalletAddr, newAddr.ID)

	return nil
}
