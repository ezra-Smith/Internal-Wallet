package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWalletAddressesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWalletAddressesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWalletAddressesLogic {
	return &ListWalletAddressesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWalletAddressesLogic) ListWalletAddresses(in *pb.ListWalletAddressesReq) (*pb.ListWalletAddressesResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	id, _ := strconv.ParseInt(uidStr, 10, 64)
	walletRepo := l.svcCtx.UserWalletAddressRepository
	asset := ""
	chain := ""
	includeWhitelist := false
	if in != nil {
		asset = in.Asset
		chain = in.Chain
		includeWhitelist = in.IncludeWhitelist
	}
	rows, _ := walletRepo.ListAddresses(l.ctx, id, asset, chain)
	// 查询白名单数据（如果需要）
	var whitelists []*model.UserWithdrawAuditWhitelistRuleModel
	if includeWhitelist && l.svcCtx.UserWithdrawAuditWhitelistRuleRepository != nil {
		whitelists, _ = l.svcCtx.UserWithdrawAuditWhitelistRuleRepository.ListActiveByUserSource(l.ctx, id, "user")
	}
	// 收集所有涉及的链（包括收款地址和白名单地址）
	chainIconMap := l.loadChainIconsWithWhitelist(rows, whitelists)
	// 构造白名单返回数据
	var mostRecentWhitelist *pb.WhitelistAddressItem
	if includeWhitelist && len(whitelists) > 0 {
		mostRecentWhitelist = buildMostRecentWhitelist(whitelists, chain, chainIconMap)
	}

	items := make([]*pb.WalletAddressItem, 0, len(rows))
	for _, w := range rows {
		items = append(items, &pb.WalletAddressItem{
			Id:        strconv.FormatInt(w.ID, 10),
			Asset:     w.Asset,
			Chain:     w.Chain,
			Address:   w.Address,
			Label:     w.Label,
			IsDefault: w.IsDefault,
			CreatedAt: w.CreatedAt.Unix(),
			ChainIcon: chainIconMap[strings.ToUpper(strings.TrimSpace(w.Chain))],
		})
	}

	return &pb.ListWalletAddressesResp{
		Success:          true,
		Items:            items,
		WhitelistAddress: mostRecentWhitelist,
	}, nil
}

// loadChainIconsWithWhitelist 加载所有涉及的链的图标（包括白名单，带 Redis 缓存）
func (l *ListWalletAddressesLogic) loadChainIconsWithWhitelist(rows []model.UserWalletAddressModel, whitelists []*model.UserWithdrawAuditWhitelistRuleModel) map[string]string {
	chainIconMap := make(map[string]string)

	if l.svcCtx.ChainRepository == nil {
		return chainIconMap
	}

	// 收集所有需要的链（包括收款地址和白名单地址）
	chainSet := make(map[string]bool)
	for _, row := range rows {
		chain := strings.ToUpper(strings.TrimSpace(row.Chain))
		if chain != "" {
			chainSet[chain] = true
		}
	}
	for _, wl := range whitelists {
		if wl != nil {
			chain := strings.ToUpper(strings.TrimSpace(wl.ChainCode))
			if chain != "" {
				chainSet[chain] = true
			}
		}
	}

	// 先从 Redis 批量查询
	cachedCount := 0
	if l.svcCtx.RedisClient != nil {
		redisCtx, redisCancel := context.WithTimeout(context.Background(), 2*time.Second)
		for chainName := range chainSet {
			cacheKey := fmt.Sprintf("chain:icon:%s", chainName)
			cached, err := l.svcCtx.RedisClient.Get(redisCtx, cacheKey).Result()
			if err == nil && cached != "" {
				chainIconMap[chainName] = cached
				cachedCount++
			}
		}
		redisCancel()
	}

	// 如果所有链都已缓存，直接返回
	if cachedCount == len(chainSet) {
		return chainIconMap
	}

	// 缓存未完全命中，一次性查询所有 chain 数据
	chains, err := l.svcCtx.ChainRepository.ListEnabledChains(l.ctx)
	if err == nil && len(chains) > 0 {
		// 构建 map 并写入缓存
		if l.svcCtx.RedisClient != nil {
			cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 2*time.Second)
			for _, chain := range chains {
				chainName := strings.ToUpper(strings.TrimSpace(chain.Name))
				if chainName == "" {
					continue
				}
				// 更新结果 map
				chainIconMap[chainName] = chain.IconUrl
				// 写入 Redis 缓存（1小时）
				if chain.IconUrl != "" {
					cacheKey := fmt.Sprintf("chain:icon:%s", chainName)
					l.svcCtx.RedisClient.Set(cacheCtx, cacheKey, chain.IconUrl, 1*time.Hour)
				}
			}
			cacheCancel()
		} else {
			// 没有 Redis，直接构建 map
			for _, chain := range chains {
				chainName := strings.ToUpper(strings.TrimSpace(chain.Name))
				if chainName != "" {
					chainIconMap[chainName] = chain.IconUrl
				}
			}
		}
	}
	return chainIconMap
}

// buildMostRecentWhitelist 查找最近更新的启用白名单地址（精简版）
func buildMostRecentWhitelist(whitelists []*model.UserWithdrawAuditWhitelistRuleModel, filterChain string, chainIconMap map[string]string) *pb.WhitelistAddressItem {
	var mostRecent *pb.WhitelistAddressItem
	var mostRecentTime int64

	filterChain = strings.ToUpper(strings.TrimSpace(filterChain))

	for _, wl := range whitelists {
		if wl == nil || !wl.Enabled {
			continue
		}

		chain := strings.TrimSpace(wl.ChainCode)
		addr := strings.TrimSpace(wl.Address)
		if chain == "" || addr == "" {
			continue
		}

		// 如果指定了链筛选，只处理该链的白名单
		if filterChain != "" && strings.ToUpper(chain) != filterChain {
			continue
		}

		updatedAt := wl.UpdatedAt.Unix()
		if updatedAt > mostRecentTime {
			mostRecentTime = updatedAt
			// 只返回关键字段：链、地址、钱包图标、链图标
			mostRecent = &pb.WhitelistAddressItem{
				Chain:     chain,
				Address:   addr,
				ChainIcon: chainIconMap[strings.ToUpper(chain)],
			}
		}
	}

	return mostRecent
}
