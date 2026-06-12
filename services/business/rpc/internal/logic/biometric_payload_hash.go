package logic

import (
	"fmt"
	"sort"
	"strings"

	"internalwallet/proto/pb"

	"github.com/shopspring/decimal"
)

type withdrawPayloadForHash struct {
	Asset   string `json:"asset"`
	Chain   string `json:"chain"`
	Amount  string `json:"amount"`
	Address string `json:"address"`
	MemoTag string `json:"memo_tag,omitempty"`
}

func calcWithdrawPayloadHash(asset, chain, amount, address, memoTag string) (string, error) {
	assetCode := normalizeCode(asset)
	chainCode := normalizeCode(chain)
	if assetCode == "" || chainCode == "" {
		return "", fmt.Errorf("asset/chain required")
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return "", fmt.Errorf("address required")
	}
	amountStr := strings.TrimSpace(amount)
	amt, err := decimal.NewFromString(amountStr)
	if err != nil || !amt.GreaterThan(decimal.Zero) {
		return "", fmt.Errorf("invalid amount")
	}
	memoTag = strings.TrimSpace(memoTag)

	return sha256HexJSON(withdrawPayloadForHash{
		Asset:   assetCode,
		Chain:   chainCode,
		Amount:  amt.String(),
		Address: address,
		MemoTag: memoTag,
	})
}

type withdrawAuditWhitelistBindItemForHash struct {
	ChainCode  string `json:"chain_code"`
	Enabled    bool   `json:"enabled"`
	Address    string `json:"address,omitempty"`
	WalletName string `json:"wallet_name,omitempty"`
	WalletIcon string `json:"wallet_icon,omitempty"`
}

type withdrawAuditWhitelistBindPayloadForHash struct {
	Items []withdrawAuditWhitelistBindItemForHash `json:"items"`
}

func calcWithdrawAuditWhitelistBindingPayloadHash(items []*pb.SetMyWithdrawAuditWhitelistBindingReqItem) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("items required")
	}
	seenChainCodes := make(map[string]struct{}, len(items))
	canon := make([]withdrawAuditWhitelistBindItemForHash, 0, len(items))

	for _, item := range items {
		if item == nil {
			return "", fmt.Errorf("invalid item")
		}
		chainCode := normalizeCode(item.ChainCode)
		if chainCode == "" {
			return "", fmt.Errorf("chain_code required")
		}
		if _, ok := seenChainCodes[chainCode]; ok {
			return "", fmt.Errorf("duplicate chain_code")
		}
		seenChainCodes[chainCode] = struct{}{}

		if !item.Enabled {
			canon = append(canon, withdrawAuditWhitelistBindItemForHash{
				ChainCode: chainCode,
				Enabled:   false,
			})
			continue
		}

		walletName, err := normalizeWithdrawAuditWhitelistWalletNameRequired(item.WalletName)
		if err != nil {
			return "", fmt.Errorf("invalid wallet_name")
		}
		walletIcon, err := normalizeWithdrawAuditWhitelistWalletIcon(item.WalletIcon)
		if err != nil {
			return "", fmt.Errorf("invalid wallet_icon")
		}

		address := normalizeWhitelistAddress(strings.TrimSpace(item.Address))
		if address == "" {
			return "", fmt.Errorf("address required")
		}
		if !isValidWalletAddress(chainCode, address) {
			return "", fmt.Errorf("invalid address")
		}

		canon = append(canon, withdrawAuditWhitelistBindItemForHash{
			ChainCode:  chainCode,
			Enabled:    true,
			Address:    address,
			WalletName: walletName,
			WalletIcon: walletIcon,
		})
	}

	sort.Slice(canon, func(i, j int) bool { return canon[i].ChainCode < canon[j].ChainCode })
	return sha256HexJSON(withdrawAuditWhitelistBindPayloadForHash{Items: canon})
}

func calcWithdrawAuditWhitelistUnbindAllPayloadHash() (string, error) {
	return sha256HexJSON(map[string]string{"action": "unbind_all_wallet_addresses"})
}
