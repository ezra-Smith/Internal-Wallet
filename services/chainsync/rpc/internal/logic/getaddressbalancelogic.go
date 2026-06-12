package logic

import (
	"context"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/chainsync/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAddressBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAddressBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAddressBalanceLogic {
	return &GetAddressBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== 余额监控 ====================
func (l *GetAddressBalanceLogic) GetAddressBalance(in *pb.GetAddressBalanceReq) (*pb.GetAddressBalanceResp, error) {
	if in == nil {
		return &pb.GetAddressBalanceResp{
			Success: false,
		}, nil
	}

	address := strings.TrimSpace(in.Address)
	if address == "" {
		l.Logger.Error("GetAddressBalance: address is empty")
		return &pb.GetAddressBalanceResp{
			Success: false,
		}, nil
	}

	chainType := in.Chain
	if chainType == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
		l.Logger.Error("GetAddressBalance: chain type is unspecified")
		return &pb.GetAddressBalanceResp{
			Success: false,
		}, nil
	}

	// 检查 ProviderPool 是否可用
	if l.svcCtx.ProviderPool == nil {
		l.Logger.Error("GetAddressBalance: ProviderPool is nil")
		return &pb.GetAddressBalanceResp{
			Success: false,
		}, nil
	}

	// 获取对应链的 provider
	provider, err := l.svcCtx.ProviderPool.GetProvider(chainType)
	if err != nil {
		l.Logger.Errorf("GetAddressBalance: failed to get provider for chain %v: %v", chainType, err)
		return &pb.GetAddressBalanceResp{
			Success: false,
		}, nil
	}

	// 调用 provider 获取余额
	tokens := in.Tokens
	if tokens == nil {
		tokens = []string{}
	}

	resp, err := provider.GetAddressBalance(l.ctx, address, tokens)
	if err != nil {
		l.Logger.Errorf("GetAddressBalance: failed to get balance for address %s on chain %v: %v", address, chainType, err)
		return &pb.GetAddressBalanceResp{
			Success: false,
		}, nil
	}

	if resp == nil {
		l.Logger.Errorf("GetAddressBalance: provider returned nil response for address %s on chain %v", address, chainType)
		return &pb.GetAddressBalanceResp{
			Success: false,
		}, nil
	}

	// 确保 Success 标志被设置
	resp.Success = true
	resp.LastUpdated = time.Now().Unix()

	l.Logger.Infof("GetAddressBalance: successfully retrieved balance for address %s on chain %v, native_balance=%s",
		address, chainType, resp.NativeBalance)

	return resp, nil
}
