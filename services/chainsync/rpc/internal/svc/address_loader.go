package svc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/config"

	"github.com/zeromicro/go-zero/core/logx"
)

// MonitorAddress 监控地址信息（内部使用）
type MonitorAddress struct {
	UserID  int64
	Chain   string
	Address string
}

// AddressLoader 地址加载器，用于从admin服务获取监控地址
type AddressLoader struct {
	config       *config.Config
	adminClient  pb.AdminClient         // 调用 Admin 服务获取充值地址
	signerClient pb.SignerServiceClient // 调用 Signer 服务获取公司钱包

	// 缓存和同步
	mu sync.RWMutex

	// 重试配置
	maxRetries int
	retryDelay time.Duration
}

// NewAddressLoader 创建地址加载器
func NewAddressLoader(cfg *config.Config) (*AddressLoader, error) {
	loader := &AddressLoader{
		config:     cfg,
		maxRetries: 3,
		retryDelay: 5 * time.Second,
	}

	// 建立与admin服务的连接（获取充值地址）
	if err := loader.connectToAdmin(); err != nil {
		logx.Errorf("Failed to connect to admin service: %v", err)
		// 不返回错误，允许后续重连
	}

	// 建立与signer服务的连接（获取公司钱包）
	if err := loader.connectToSigner(); err != nil {
		logx.Errorf("Failed to connect to signer service: %v", err)
		// 不返回错误，允许后续重连
	}

	return loader, nil
}

// LoadDepositAddresses 加载充值地址（分页）
func (al *AddressLoader) LoadDepositAddresses(ctx context.Context, chains []string, status int32, pageSize int32) ([]*MonitorAddress, int32, error) {
	var allAddresses []*MonitorAddress
	var total int64
	page := int32(1)

	// 如果pageSize为0，使用默认值500
	if pageSize <= 0 {
		pageSize = 500
	}

	// 将 status 转换为字符串（Admin 接口需要）
	var statusStr string
	if status > 0 {
		statusStr = "active" // 只监控活跃地址
	}

	logx.Infof("Loading deposit addresses from Admin: chains=%v, status=%s, pageSize=%d", chains, statusStr, pageSize)

	for {
		// 构建 Admin 服务的请求
		req := &pb.ListDepositAddressesRequest{
			Page:     page,
			PageSize: pageSize,
			Status:   statusStr,
		}

		// 如果指定了链，只取第一个链（Admin接口是chain_code单值）
		if len(chains) > 0 {
			req.ChainCode = chains[0]
		}

		// 调用admin服务的ListDepositAddresses接口
		resp, err := al.listAdminDepositAddressesWithRetry(ctx, req)

		if err != nil {
			return nil, 0, fmt.Errorf("failed to list deposit addresses from admin: %v", err)
		}

		// 检查响应状态
		if !resp.Success {
			return nil, 0, fmt.Errorf("admin service error: %s", resp.Message)
		}

		// 转换 Admin 的返回格式为 MonitorAddress
		for _, item := range resp.Data.Addresses {
			addr := &MonitorAddress{
				UserID:  item.UserId,
				Chain:   item.ChainCode,
				Address: item.Address,
			}
			allAddresses = append(allAddresses, addr)
		}

		// 更新总数（第一次获取时设置）
		if page == 1 && resp.Pagination != nil {
			total = resp.Pagination.Total
		}

		logx.Infof("Loaded page %d: %d addresses, total so far: %d", page, len(resp.Data.Addresses), len(allAddresses))

		// 检查是否还有更多页
		//
		// 现实情况：
		// - Admin 端分页字段（尤其是 TotalPages）可能不可靠（例如 total>0 但 total_pages=0）
		// - Admin 端也可能对每页条数做了上限（比如无论请求 PageSize=500，实际只返回 100）
		//
		// 因此这里不再用 `len(page)<pageSize` 判断末页，而是：
		// - 若 total 可用：只要累计 < total 就继续翻页
		// - 否则：直到返回空列表才停止
		if len(resp.Data.Addresses) == 0 {
			break
		}
		if total > 0 && int64(len(allAddresses)) >= total {
			break
		}

		page++

		// 添加短暂延迟，避免对admin服务造成过大压力
		time.Sleep(100 * time.Millisecond)
	}

	logx.Infof("Finished loading deposit addresses from Admin: total=%d, loaded=%d", total, len(allAddresses))
	return allAddresses, int32(total), nil
}

// LoadCompanyWallets 加载公司钱包地址
func (al *AddressLoader) LoadCompanyWallets(ctx context.Context, chain string, temperature, status int32, addressType string) ([]*pb.CompanyWalletInfo, error) {
	logx.Infof("Loading company wallets: chain=%s, temperature=%d, status=%d, addressType=%s",
		chain, temperature, status, addressType)

	// 调用signer服务的ListCompanyWallets接口
	resp, err := al.listCompanyWalletsWithRetry(ctx, &pb.ListCompanyWalletsRequest{
		Chain:       chain,
		Temperature: temperature,
		Status:      status,
		AddressType: addressType,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list company wallets: %v", err)
	}

	// 检查响应状态
	if resp.Code != 0 {
		return nil, fmt.Errorf("signer service error: %s", resp.Message)
	}

	logx.Infof("Loaded company wallets: total=%d", len(resp.Wallets))
	return resp.Wallets, nil
}

// listAdminDepositAddressesWithRetry 带重试机制的充值地址列表获取（从Admin服务）
func (al *AddressLoader) listAdminDepositAddressesWithRetry(ctx context.Context, req *pb.ListDepositAddressesRequest) (*pb.ListDepositAddressesResponse, error) {
	var lastErr error

	for i := 0; i < al.maxRetries; i++ {
		// 确保连接可用
		if err := al.ensureAdminConnection(); err != nil {
			lastErr = err
			logx.Errorf("Failed to ensure connection to admin (attempt %d/%d): %v", i+1, al.maxRetries, err)
			time.Sleep(al.retryDelay)
			continue
		}

		// 调用admin服务
		resp, err := al.adminClient.ListDepositAddresses(al.withAdminAuth(ctx), req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		logx.Errorf("Failed to call Admin.ListDepositAddresses (attempt %d/%d): %v", i+1, al.maxRetries, err)

		// 如果不是最后一次尝试，等待后重试
		if i < al.maxRetries-1 {
			time.Sleep(al.retryDelay)
		}
	}

	return nil, lastErr
}

// listCompanyWalletsWithRetry 带重试机制的公司钱包列表获取
func (al *AddressLoader) listCompanyWalletsWithRetry(ctx context.Context, req *pb.ListCompanyWalletsRequest) (*pb.ListCompanyWalletsResponse, error) {
	var lastErr error

	for i := 0; i < al.maxRetries; i++ {
		// 确保连接可用
		if err := al.ensureSignerConnection(); err != nil {
			lastErr = err
			logx.Errorf("Failed to ensure connection (attempt %d/%d): %v", i+1, al.maxRetries, err)
			time.Sleep(al.retryDelay)
			continue
		}

		// 调用远程服务
		resp, err := al.signerClient.ListCompanyWallets(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		logx.Errorf("Failed to call ListCompanyWallets (attempt %d/%d): %v", i+1, al.maxRetries, err)

		// 如果不是最后一次尝试，等待后重试
		if i < al.maxRetries-1 {
			time.Sleep(al.retryDelay)
		}
	}

	return nil, lastErr
}

// LoadVaultAddresses 加载 Vault 地址池（分页）
func (al *AddressLoader) LoadVaultAddresses(ctx context.Context, status string, pageSize int32) ([]*MonitorAddress, int32, error) {
	var allAddresses []*MonitorAddress
	var total int64
	page := int32(1)

	if pageSize <= 0 {
		pageSize = 500
	}

	statusStr := strings.TrimSpace(status)
	if statusStr == "" {
		statusStr = "active"
	}

	logx.Infof("Loading vault addresses from Admin: status=%s, pageSize=%d", statusStr, pageSize)

	for {
		req := &pb.ListVaultAddressesRequest{
			Page:     page,
			PageSize: pageSize,
			Status:   statusStr,
		}

		resp, err := al.listAdminVaultAddressesWithRetry(ctx, req)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list vault addresses from admin: %v", err)
		}
		if resp == nil || !resp.Success {
			msg := ""
			if resp != nil {
				msg = resp.Message
			}
			return nil, 0, fmt.Errorf("admin service error: %s", msg)
		}

		var items []*pb.VaultAddressItem
		if resp.Data != nil {
			items = resp.Data.Addresses
		}
		for _, item := range items {
			if item == nil {
				continue
			}
			chain := strings.TrimSpace(item.Network)
			address := strings.TrimSpace(item.Address)
			if chain == "" || address == "" {
				continue
			}
			allAddresses = append(allAddresses, &MonitorAddress{
				UserID:  0,
				Chain:   chain,
				Address: address,
			})
		}

		if page == 1 && resp.Data != nil && resp.Data.Pagination != nil {
			total = resp.Data.Pagination.Total
		}

		// Pagination fields (especially TotalPages) may be unreliable.
		// Prefer "stop on empty page", or stop when accumulated >= total if total is present.
		if len(items) == 0 {
			break
		}
		if total > 0 && int64(len(allAddresses)) >= total {
			break
		}

		page++
		time.Sleep(100 * time.Millisecond)
	}

	logx.Infof("Finished loading vault addresses from Admin: total=%d, loaded=%d", total, len(allAddresses))
	return allAddresses, int32(total), nil
}

func (al *AddressLoader) listAdminVaultAddressesWithRetry(ctx context.Context, req *pb.ListVaultAddressesRequest) (*pb.ListVaultAddressesResponse, error) {
	var lastErr error
	for i := 0; i < al.maxRetries; i++ {
		if err := al.ensureAdminConnection(); err != nil {
			lastErr = err
			logx.Errorf("Failed to ensure connection to admin (attempt %d/%d): %v", i+1, al.maxRetries, err)
			time.Sleep(al.retryDelay)
			continue
		}

		resp, err := al.adminClient.ListVaultAddresses(al.withAdminAuth(ctx), req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		logx.Errorf("Failed to call Admin.ListVaultAddresses (attempt %d/%d): %v", i+1, al.maxRetries, err)
		if i < al.maxRetries-1 {
			time.Sleep(al.retryDelay)
		}
	}
	return nil, lastErr
}

// LoadWeb3UserAddresses 加载 Web3 用户地址（分页）
func (al *AddressLoader) LoadWeb3UserAddresses(ctx context.Context, status string, pageSize int32) ([]*MonitorAddress, int32, error) {
	var allAddresses []*MonitorAddress
	var total int64
	page := int32(1)

	if pageSize <= 0 {
		pageSize = 500
	}

	statusStr := strings.TrimSpace(status)
	if statusStr == "" {
		statusStr = "enabled"
	}

	logx.Infof("Loading web3 user addresses from Admin: status=%s, pageSize=%d", statusStr, pageSize)

	for {
		req := &pb.ListWeb3UserAddressesRequest{
			Page:     page,
			PageSize: pageSize,
			Status:   statusStr,
		}

		resp, err := al.listAdminWeb3UserAddressesWithRetry(ctx, req)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to list web3 user addresses from admin: %v", err)
		}
		if resp == nil || !resp.Success {
			msg := ""
			if resp != nil {
				msg = resp.Message
			}
			return nil, 0, fmt.Errorf("admin service error: %s", msg)
		}

		var items []*pb.Web3UserAddressItem
		if resp.Data != nil {
			items = resp.Data.Addresses
		}
		for _, item := range items {
			if item == nil {
				continue
			}
			network := strings.TrimSpace(item.Network)
			address := strings.TrimSpace(item.Address)
			if network == "" || address == "" {
				continue
			}
			allAddresses = append(allAddresses, &MonitorAddress{
				UserID:  item.UserId,
				Chain:   network,
				Address: address,
			})
		}

		if page == 1 && resp.Data != nil && resp.Data.Pagination != nil {
			total = resp.Data.Pagination.Total
		}

		// Pagination fields (especially TotalPages) may be unreliable.
		// Prefer "stop on empty page", or stop when accumulated >= total if total is present.
		if len(items) == 0 {
			break
		}
		if total > 0 && int64(len(allAddresses)) >= total {
			break
		}

		page++
		time.Sleep(100 * time.Millisecond)
	}

	logx.Infof("Finished loading web3 user addresses from Admin: total=%d, loaded=%d", total, len(allAddresses))
	return allAddresses, int32(total), nil
}

func (al *AddressLoader) listAdminWeb3UserAddressesWithRetry(ctx context.Context, req *pb.ListWeb3UserAddressesRequest) (*pb.ListWeb3UserAddressesResponse, error) {
	var lastErr error
	for i := 0; i < al.maxRetries; i++ {
		if err := al.ensureAdminConnection(); err != nil {
			lastErr = err
			logx.Errorf("Failed to ensure connection to admin (attempt %d/%d): %v", i+1, al.maxRetries, err)
			time.Sleep(al.retryDelay)
			continue
		}

		resp, err := al.adminClient.ListWeb3UserAddresses(al.withAdminAuth(ctx), req)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		logx.Errorf("Failed to call Admin.ListWeb3UserAddresses (attempt %d/%d): %v", i+1, al.maxRetries, err)
		if i < al.maxRetries-1 {
			time.Sleep(al.retryDelay)
		}
	}
	return nil, lastErr
}
