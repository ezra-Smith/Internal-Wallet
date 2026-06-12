package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateWalletAddressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateWalletAddressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateWalletAddressLogic {
	return &ValidateWalletAddressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateWalletAddressLogic) ValidateWalletAddress(in *pb.ValidateWalletAddressReq) (*pb.ValidateWalletAddressResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("invalid params")
	}
	asset := strings.TrimSpace(in.Asset)
	chain := strings.TrimSpace(in.Chain)
	address := strings.TrimSpace(in.Address)
	if asset == "" || chain == "" || address == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	valid := false
	valid = isValidWalletAddress(chain, address)

	isInternal := false
	if l.svcCtx != nil && l.svcCtx.ChainSyncRpc != nil {
		chainType := chainCodeToChainSyncType(chain)
		if chainType != pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
			if resp, err := l.svcCtx.ChainSyncRpc.IsInternalAddress(l.ctx, &pb.IsInternalAddressReq{
				Chain:   chainType,
				Address: address,
			}); err != nil {
				l.Errorf("ChainSync IsInternalAddress failed: chain=%s addr=%s err=%v", chain, address, err)
			} else if resp != nil && resp.Success {
				isInternal = resp.IsInternal
			}
		}
	}

	// Fallback: deposit address table only (best-effort).
	if !isInternal && l.svcCtx != nil && l.svcCtx.WalletDepositAddressRepository != nil {
		if _, err := l.svcCtx.WalletDepositAddressRepository.FindActiveByAddress(l.ctx, address); err == nil {
			isInternal = true
		}
	}

	needMemo := false
	switch strings.ToUpper(asset) {
	case "XRP", "XLM", "EOS":
		needMemo = true
	}

	if l.svcCtx != nil && l.svcCtx.ChainRepository != nil {
		_, _ = l.svcCtx.ChainRepository.FindByCondition(l.ctx, map[string]interface{}{"name = ?": chain})
	}

	return &pb.ValidateWalletAddressResp{
		Success:     true,
		Valid:       valid,
		IsInternal:  isInternal,
		NeedMemoTag: needMemo,
		Message: func() string {
			if valid {
				return "ok"
			} else {
				return "invalid address"
			}
		}(),
	}, nil
}

func chainCodeToChainSyncType(chain string) pb.BlockChainType {
	switch strings.ToUpper(strings.TrimSpace(chain)) {
	case "ETH", "ETHEREUM":
		return pb.BlockChainType_CHAIN_TYPE_ETHEREUM
	case "BSC", "BNB", "BNBCHAIN":
		return pb.BlockChainType_CHAIN_TYPE_BSC
	case "TRN", "TRON", "TRX":
		return pb.BlockChainType_CHAIN_TYPE_TRON
	default:
		return pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED
	}
}
