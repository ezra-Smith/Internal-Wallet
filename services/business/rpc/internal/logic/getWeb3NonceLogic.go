package logic

import (
	"context"
	_ "fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"
)

type GetWeb3NonceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3NonceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3NonceLogic {
	return &GetWeb3NonceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetWeb3Nonce 获取 Web3 用户地址的 Nonce（用于构建 EVM 交易）
func (l *GetWeb3NonceLogic) GetWeb3Nonce(in *pb.GetWeb3NonceReq) (*pb.GetWeb3NonceResp, error) {
	if in == nil {
		return &pb.GetWeb3NonceResp{
			Success: false,
			Message: "请求参数不能为空",
		}, nil
	}

	// 验证必填参数
	deviceID := strings.TrimSpace(in.DeviceId)
	address := strings.TrimSpace(in.Address)
	network := strings.TrimSpace(in.Network)

	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}
	if address == "" {
		return nil, errx.Web3AddressRequired()
	}
	if network == "" {
		return nil, errx.Web3NetworkRequired()
	}

	// 1. 验证地址是否存在（不验证用户归属，因为地址可以通过助记词导入到不同设备）
	// 注意：余额记录是地址级别的，不绑定到特定用户/设备
	web3UserAddress, err := l.svcCtx.Web3UserAddressRepository.FindByAddress(l.ctx, address)
	if err != nil || web3UserAddress == nil {
		// 如果地址不存在，仍然允许查询 nonce（可能是新导入的地址）
		l.Infof("地址未在系统中注册: address=%s, network=%s (允许继续查询 nonce)", address, network)
	} else {
		// 检查地址是否启用
		if !web3UserAddress.Enabled {
			return nil, errx.Web3AddressDisabled()
		}
	}

	// 3. 转换网络名称到 ChainRpcType
	chainType := l.mapNetworkToChainType(network)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return nil, errx.Web3UnsupportedNetwork(network)
	}

	// 4. 调用 ChainRPC 获取 nonce
	if l.svcCtx.ChainRpc == nil {
		l.Errorf("ChainRpc客户端未初始化")
		return nil, errx.Web3ChainServiceUnavailable()
	}

	// 处理 pending 参数
	// 注意：proto 中 bool 类型的默认值是 false，我们无法区分"未设置"和"明确设置为 false"
	// 按照最佳实践，构建交易时应查询 pending nonce（包含待确认交易），以避免 nonce 冲突
	// 因此，为了构建交易时的最佳实践，默认查询 pending nonce
	// 当前实现：无论 pending 参数如何，都查询 pending nonce，确保包含待确认交易，避免 nonce 冲突
	// 这是构建交易时的最佳实践，因为使用 pending nonce 可以避免与待确认交易发生 nonce 冲突
	pending := true // 始终查询 pending nonce，以确保包含待确认交易，避免 nonce 冲突

	getNonceReq := &pb.GetNonceReq{
		Chain:   chainType,
		Address: address,
		Pending: pending,
	}

	getNonceResp, err := l.svcCtx.ChainRpc.GetNonce(l.ctx, getNonceReq)
	if err != nil || getNonceResp == nil || !getNonceResp.Success {
		l.Errorf("获取 nonce 失败: address=%s, network=%s, error=%v", address, network, err)
		return nil, errx.Web3ChainServiceUnavailable()
	}

	return &pb.GetWeb3NonceResp{
		Success:      true,
		Message:      "ok",
		Nonce:        getNonceResp.Nonce,
		PendingNonce: getNonceResp.PendingNonce,
	}, nil
}

// mapNetworkToChainType 将网络名称映射到 ChainRpcType
func (l *GetWeb3NonceLogic) mapNetworkToChainType(network string) pb.ChainRpcType {
	networkLower := strings.ToLower(network)
	switch networkLower {
	case "ethereum", "eth":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case "bsc", "binance", "binance smart chain":
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	case "tron", "trx":
		return pb.ChainRpcType_CHAIN_TYPE_TRON
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED
	}
}
