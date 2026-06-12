package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetNonceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNonceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNonceLogic {
	return &GetNonceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetNonce 查询账户的 Nonce 值
// 对于 EVM 兼容链（ETH/BSC），返回账户的 nonce
// 对于 TRON，返回 0（TRON 不使用 nonce）
func (l *GetNonceLogic) GetNonce(in *pb.GetNonceReq) (*pb.GetNonceResp, error) {
	if in == nil {
		return &pb.GetNonceResp{
			Success: false,
			Message: "请求参数不能为空",
		}, nil
	}

	// 验证链类型
	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return &pb.GetNonceResp{
			Success: false,
			Message: "链类型不能为空",
		}, nil
	}

	// 验证地址
	address := strings.TrimSpace(in.Address)
	if address == "" {
		return &pb.GetNonceResp{
			Success: false,
			Message: "地址不能为空",
		}, nil
	}

	// 标准化地址格式（EVM 链需要 0x 前缀）
	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_ETHEREUM || in.Chain == pb.ChainRpcType_CHAIN_TYPE_BSC {
		if !strings.HasPrefix(address, "0x") {
			address = "0x" + address
		}
		address = strings.ToLower(address)
	}

	// 根据链类型获取 nonce
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getEVMNonce(in.Chain, address, in.Pending)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		// TRON 不使用 nonce，返回 0
		return &pb.GetNonceResp{
			Success:      true,
			Message:      "TRON 不使用 nonce，返回 0",
			Nonce:        "0",
			PendingNonce: "0",
		}, nil
	default:
		return &pb.GetNonceResp{
			Success: false,
			Message: fmt.Sprintf("不支持的链类型: %v", in.Chain),
		}, nil
	}
}

// getEVMNonce 获取 EVM 兼容链的 nonce
func (l *GetNonceLogic) getEVMNonce(chainType pb.ChainRpcType, address string, pending bool) (*pb.GetNonceResp, error) {
	// 获取 EVM 客户端
	client, err := l.svcCtx.ChainMgr.GetETHClient(chainType)
	if err != nil {
		l.Errorf("获取 EVM 客户端失败: chain=%v, error=%v", chainType, err)
		return &pb.GetNonceResp{
			Success: false,
			Message: fmt.Sprintf("获取链客户端失败: %v", err),
		}, nil
	}

	// 解析地址
	addr := common.HexToAddress(address)

	// 为以太坊 RPC 调用设置独立的超时上下文（10秒，避免以太坊节点响应慢导致的问题）
	ctxWithTimeout, cancel := context.WithTimeout(l.ctx, 10*time.Second)
	defer cancel()

	// 获取 pending nonce（包含待确认交易）
	pendingNonce, err := client.PendingNonceAt(ctxWithTimeout, addr)
	if err != nil {
		l.Errorf("获取 pending nonce 失败: address=%s, error=%v", address, err)
		// 检查是否是超时错误
		if ctxWithTimeout.Err() == context.DeadlineExceeded {
			return &pb.GetNonceResp{
				Success: false,
				Message: "获取 pending nonce 超时，请稍后重试",
			}, nil
		}
		return &pb.GetNonceResp{
			Success: false,
			Message: fmt.Sprintf("获取 pending nonce 失败: %v", err),
		}, nil
	}

	// 如果只需要 pending nonce，直接返回
	if pending {
		return &pb.GetNonceResp{
			Success:      true,
			Message:      "查询成功",
			Nonce:        strconv.FormatUint(pendingNonce, 10),
			PendingNonce: strconv.FormatUint(pendingNonce, 10),
		}, nil
	}

	// 获取已确认的 nonce（不包含待确认交易）
	// 为 confirmed nonce 查询设置独立的超时上下文
	ctxWithTimeout2, cancel2 := context.WithTimeout(l.ctx, 10*time.Second)
	defer cancel2()

	confirmedNonce, err := client.NonceAt(ctxWithTimeout2, addr, nil) // nil 表示最新区块
	if err != nil {
		l.Errorf("获取 confirmed nonce 失败: address=%s, error=%v", address, err)
		// 如果获取 confirmed nonce 失败，使用 pending nonce 作为 fallback
		return &pb.GetNonceResp{
			Success:      true,
			Message:      "获取 confirmed nonce 失败，返回 pending nonce",
			Nonce:        strconv.FormatUint(pendingNonce, 10),
			PendingNonce: strconv.FormatUint(pendingNonce, 10),
		}, nil
	}

	return &pb.GetNonceResp{
		Success:      true,
		Message:      "查询成功",
		Nonce:        strconv.FormatUint(confirmedNonce, 10),
		PendingNonce: strconv.FormatUint(pendingNonce, 10),
	}, nil
}
