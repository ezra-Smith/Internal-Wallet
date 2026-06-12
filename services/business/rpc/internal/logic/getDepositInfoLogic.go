package logic

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDepositInfoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDepositInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDepositInfoLogic {
	return &GetDepositInfoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetDepositInfoLogic) GetDepositInfo(in *pb.GetDepositInfoReq) (*pb.GetDepositInfoResp, error) {
	var address []*pb.DepositAddressItem
	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	if l.svcCtx.WalletDepositAddressRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	// Parse request parameters
	targetChain := ""
	if in != nil && strings.TrimSpace(in.Asset) != "" {
		targetChain = strings.ToUpper(strings.TrimSpace(in.Asset))
	}
	assetCode := ""
	if in != nil && strings.TrimSpace(in.AssetCode) != "" {
		assetCode = strings.ToUpper(strings.TrimSpace(in.AssetCode))
	}

	// Step 1: Get user's deposit addresses
	rows, _ := l.svcCtx.WalletDepositAddressRepository.ListActiveByUser(l.ctx, uid)
	chainToAddress := make(map[string]string)
	for _, r := range rows {
		if r == nil || strings.TrimSpace(r.Address) == "" {
			continue
		}
		chain := strings.ToUpper(strings.TrimSpace(r.ChainCode))
		if chain == "" {
			continue
		}
		// Repo returns DESC by id; keep first per chain.
		if _, exists := chainToAddress[chain]; exists {
			continue
		}
		chainToAddress[chain] = strings.TrimSpace(r.Address)
	}

	// Step 2: Load chain info (chain_name, icon_url) from chain table
	chainInfoMap := make(map[string]*model.ChainModel)
	if l.svcCtx.ChainRepository != nil {
		for ch := range chainToAddress {
			if chainInfo, err := l.svcCtx.ChainRepository.FindByName(l.ctx, ch); err == nil && chainInfo != nil {
				chainInfoMap[ch] = chainInfo
			}
		}
	}

	// Step 3: Load min_deposit_amount from currency_chain_settings (if asset_code provided)
	minDepositMap := make(map[string]string)
	if assetCode != "" && l.svcCtx.CurrencyChainSettingsRepository != nil {
		for ch := range chainToAddress {
			if setting, err := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(l.ctx, assetCode, ch); err == nil && setting != nil {
				if setting.MinDepositAmount != nil && strings.TrimSpace(*setting.MinDepositAmount) != "" {
					minDepositMap[ch] = strings.TrimSpace(*setting.MinDepositAmount) + " " + assetCode
				}
			}
		}
	}

	// Step 4: Build response
	chains := make([]string, 0, len(chainToAddress))
	for ch := range chainToAddress {
		if targetChain == "" || ch == targetChain {
			chains = append(chains, ch)
		}
	}
	sort.Strings(chains)

	for _, ch := range chains {
		if strings.TrimSpace(chainToAddress[ch]) == "" {
			continue
		}

		item := &pb.DepositAddressItem{
			Asset:   ch,
			Address: chainToAddress[ch],
		}

		// Fill chain_name and icon_url from chain info
		if chainInfo, ok := chainInfoMap[ch]; ok {
			// Prefer Network as chain_name (e.g., "TRC20"), fallback to Name (e.g., "TRON")
			if strings.TrimSpace(chainInfo.Network) != "" {
				item.ChainName = strings.TrimSpace(chainInfo.Network)
			} else {
				item.ChainName = strings.TrimSpace(chainInfo.Name)
			}
			item.IconUrl = strings.TrimSpace(chainInfo.IconUrl)
		}

		// Fill min_deposit from currency_chain_settings
		if minDeposit, ok := minDepositMap[ch]; ok {
			item.MinDeposit = minDeposit
		}

		address = append(address, item)
	}

	return &pb.GetDepositInfoResp{Success: true, Address: address}, nil
}
