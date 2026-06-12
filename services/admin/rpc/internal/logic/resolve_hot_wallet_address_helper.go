package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/svc"
)

// resolveCompanyHotWalletAddress 通过 Signer 服务获取公司热钱包地址
func resolveCompanyHotWalletAddress(ctx context.Context, svcCtx *svc.ServiceContext, chainCode string) (string, error) {
	if svcCtx == nil || svcCtx.SignerRpc == nil {
		return "", fmt.Errorf("signer rpc not configured")
	}
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return "", fmt.Errorf("empty chain code")
	}
	resp, err := svcCtx.SignerRpc.GetCompanyWallet(ctx, &pb.GetCompanyWalletRequest{
		Chain:       chainCode,
		AddressType: "hot_primary",
		Temperature: 1, // hot wallet
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty signer response")
	}
	if resp.Code != 0 {
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
