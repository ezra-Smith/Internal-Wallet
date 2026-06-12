package logic

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type BuildTronTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBuildTronTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuildTronTransactionLogic {
	return &BuildTronTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BuildTronTransaction 构建 TRON 交易（返回未签名的 rawData）
func (l *BuildTronTransactionLogic) BuildTronTransaction(in *pb.ChainRpcBuildTronTransactionReq) (*pb.ChainRpcBuildTronTransactionResp, error) {
	if in == nil {
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: "request is required",
		}, nil
	}

	// 验证必填参数
	fromAddress := strings.TrimSpace(in.FromAddress)
	toAddress := strings.TrimSpace(in.ToAddress)
	amountStr := strings.TrimSpace(in.Amount)

	if fromAddress == "" {
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: "from_address is required",
		}, nil
	}

	if toAddress == "" {
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: "to_address is required",
		}, nil
	}

	if amountStr == "" {
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: "amount is required",
		}, nil
	}

	// 验证地址格式
	if !isValidTronAddress(fromAddress) {
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: fmt.Sprintf("invalid from_address: %s", fromAddress),
		}, nil
	}

	if !isValidTronAddress(toAddress) {
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: fmt.Sprintf("invalid to_address: %s", toAddress),
		}, nil
	}

	// 解析金额（TRX 使用 SUN 作为单位，1 TRX = 1,000,000 SUN；TRC20 使用代币最小单位）
	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: fmt.Sprintf("invalid amount: %s", amountStr),
		}, nil
	}

	if amount.Sign() <= 0 {
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: "amount must be greater than 0",
		}, nil
	}

	// 获取 TRON 客户端
	var tronClient *client.GrpcClient
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if err != nil {
		l.Errorf("failed to get TRON client: %v", err)
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: fmt.Sprintf("failed to get TRON client: %v", err),
		}, nil
	}
	defer tronClient.Stop()

	// 检查是否为 TRC20 代币转账
	contractAddress := strings.TrimSpace(in.ContractAddress)
	var tx *api.TransactionExtention

	if contractAddress == "" {
		// TRX 主币转账
		tx, err = tronClient.Transfer(fromAddress, toAddress, amount.Int64())
		if err != nil {
			l.Errorf("failed to create TRON transfer: %v", err)
			return &pb.ChainRpcBuildTronTransactionResp{
				Success: false,
				Message: fmt.Sprintf("failed to create TRON transfer: %v", err),
			}, nil
		}
	} else {
		// TRC20 代币转账
		// 验证合约地址格式
		if !isValidTronAddress(contractAddress) {
			return &pb.ChainRpcBuildTronTransactionResp{
				Success: false,
				Message: fmt.Sprintf("invalid contract_address: %s", contractAddress),
			}, nil
		}

		// 设置能量限制（类似于 gas limit，默认 100 TRX）
		feeLimit := int64(100000000) // 100 TRX

		// 调用 TRC20Send 方法创建 TRC20 转账交易
		tx, err = tronClient.TRC20Send(fromAddress, toAddress, contractAddress, amount, feeLimit)
		if err != nil {
			l.Errorf("failed to create TRC20 transfer: %v", err)
			return &pb.ChainRpcBuildTronTransactionResp{
				Success: false,
				Message: fmt.Sprintf("failed to create TRC20 transfer: %v", err),
			}, nil
		}
	}

	// 将 Transaction 对象序列化为 protobuf 字节
	txBytes, err := proto.Marshal(tx.Transaction)
	if err != nil {
		l.Errorf("failed to marshal TRON transaction: %v", err)
		return &pb.ChainRpcBuildTronTransactionResp{
			Success: false,
			Message: fmt.Sprintf("failed to marshal transaction: %v", err),
		}, nil
	}

	// 将字节数组编码为 hex 字符串
	rawDataHex := hexutil.Encode(txBytes)

	return &pb.ChainRpcBuildTronTransactionResp{
		Success: true,
		Message: "Transaction built successfully",
		RawData: rawDataHex,
	}, nil
}
