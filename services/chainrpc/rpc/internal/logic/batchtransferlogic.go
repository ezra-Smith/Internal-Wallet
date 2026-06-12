package logic

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type BatchTransferLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBatchTransferLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchTransferLogic {
	return &BatchTransferLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// isValidTronAddress 验证TRON地址格式
func (l *BatchTransferLogic) isValidTronAddress(addr string) bool {
	if addr == "" {
		return false
	}

	// TRON地址应该是Base58格式且以'T'开头
	if !strings.HasPrefix(addr, "T") {
		return false
	}

	// 检查长度（TRON Base58地址通常是33-35个字符）
	if len(addr) < 33 || len(addr) > 35 {
		return false
	}

	return true
}

// validateBatchTransferReq 验证批量转账请求参数
func (l *BatchTransferLogic) validateBatchTransferReq(in *pb.BatchTransferReq) error {
	if in == nil {
		return fmt.Errorf("request is nil")
	}

	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return fmt.Errorf("chain type is required")
	}

	// 验证发送地址
	if in.FromAddress == "" {
		return fmt.Errorf("from address is required")
	}

	// 检查转账记录列表
	if len(in.Transfers) == 0 {
		return fmt.Errorf("transfers list cannot be empty")
	}

	// 根据链类型验证发送地址格式
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		// 验证以太坊地址格式
		if !common.IsHexAddress(in.FromAddress) {
			return fmt.Errorf("invalid from address format: %s", in.FromAddress)
		}
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		// 验证TRON地址格式
		if !l.isValidTronAddress(in.FromAddress) {
			return fmt.Errorf("invalid from TRON address: %s", in.FromAddress)
		}
	default:
		return fmt.Errorf("unsupported chain type: %v", in.Chain)
	}

	// 验证每个转账记录
	for i, transfer := range in.Transfers {
		if transfer == nil {
			return fmt.Errorf("transfer at index %d cannot be nil", i)
		}

		// 验证to地址
		if transfer.ToAddress == "" {
			return fmt.Errorf("to address at index %d cannot be empty", i)
		}

		// 验证金额
		if transfer.Amount == "" {
			return fmt.Errorf("amount at index %d cannot be empty", i)
		}

		// 验证金额格式
		_, ok := new(big.Int).SetString(transfer.Amount, 10)
		if !ok {
			return fmt.Errorf("invalid amount format at index %d: %s", i, transfer.Amount)
		}

		// 根据链类型验证接收地址格式
		switch in.Chain {
		case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
			// 验证以太坊地址格式
			if !common.IsHexAddress(transfer.ToAddress) {
				return fmt.Errorf("invalid to address format at index %d: %s", i, transfer.ToAddress)
			}
		case pb.ChainRpcType_CHAIN_TYPE_TRON:
			// 验证TRON地址格式
			if !l.isValidTronAddress(transfer.ToAddress) {
				return fmt.Errorf("invalid to TRON address at index %d: %s", i, transfer.ToAddress)
			}
		}
	}

	return nil
}

// BatchTransfer 批量转账交易
func (l *BatchTransferLogic) BatchTransfer(in *pb.BatchTransferReq) (*pb.BatchTransferResp, error) {
	// 参数验证
	if err := l.validateBatchTransferReq(in); err != nil {
		return &pb.BatchTransferResp{
			Success:      false,
			Message:      fmt.Sprintf("validation failed: %v", err),
			Transactions: []*pb.TransactionResp{},
			Total:        0,
		}, nil
	}

	// 记录开始时间
	startTime := time.Now()
	l.Logger.Infof("Starting batch transfer for %d transactions from %s", len(in.Transfers), in.FromAddress)

	// 初始化结果切片
	transactions := make([]*pb.TransactionResp, 0, len(in.Transfers))
	successCount := 0
	failedCount := 0

	// 并发处理转账（限制并发数以避免过载）
	const maxConcurrency = 5
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 创建单个转账逻辑实例
	singleLogic := NewNativeTransferLogic(l.ctx, l.svcCtx)

	// 为每个转账创建处理任务
	for i, transfer := range in.Transfers {
		wg.Add(1)
		go func(index int, t *pb.TransferItem) {
			defer wg.Done()
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			// 创建单个转账请求
			singleReq := &pb.NativeTransferReq{
				Chain:       in.Chain,
				FromAddress: in.FromAddress,
				ToAddress:   t.ToAddress,
				Amount:      t.Amount,
			}

			// 如果有统一的gas价格，设置它
			if in.GasPrice != "" {
				if gasPrice, ok := new(big.Int).SetString(in.GasPrice, 10); ok {
					singleReq.GasPrice = gasPrice.Uint64()
				}
			}

			// 调用单个转账函数
			l.Logger.Debugf("Processing transfer %d/%d: %s -> %s (%s)",
				index+1, len(in.Transfers), in.FromAddress, t.ToAddress, t.Amount)

			resp, err := singleLogic.NativeTransfer(singleReq)

			mu.Lock()
			if err != nil {
				// 转账失败，创建失败记录
				failedTx := &pb.TransactionResp{
					Success:       false,
					Message:       fmt.Sprintf("Transfer failed: %v", err),
					TxHash:        "",
					GasUsed:       0,
					Status:        pb.TxStatus_TX_STATUS_FAILED,
					BroadcastedAt: time.Now().Unix(),
					Nonce:         0,
				}
				transactions = append(transactions, failedTx)
				failedCount++
				l.Logger.Errorf("Transfer %d failed: %s -> %s, error: %v",
					index+1, in.FromAddress, t.ToAddress, err)
			} else if resp.Success {
				// 转账成功，添加到结果中
				transactions = append(transactions, resp)
				successCount++
				l.Logger.Infof("Transfer %d succeeded: %s -> %s, tx: %s",
					index+1, in.FromAddress, t.ToAddress, resp.TxHash)
			} else {
				// 转账失败但返回了响应
				failedTx := &pb.TransactionResp{
					Success:       false,
					Message:       resp.Message,
					TxHash:        resp.TxHash,
					GasUsed:       resp.GasUsed,
					Status:        pb.TxStatus_TX_STATUS_FAILED,
					BroadcastedAt: resp.BroadcastedAt,
					Nonce:         resp.Nonce,
				}
				transactions = append(transactions, failedTx)
				failedCount++
				l.Logger.Errorf("Transfer %d failed: %s -> %s, reason: %s",
					index+1, in.FromAddress, t.ToAddress, resp.Message)
			}
			mu.Unlock()
		}(i, transfer)
	}

	// 等待所有转账完成
	wg.Wait()

	// 记录结束时间
	duration := time.Since(startTime)
	l.Logger.Infof("Batch transfer completed in %v: %d success, %d failed",
		duration, successCount, failedCount)

	// 构建响应
	success := failedCount == 0
	message := fmt.Sprintf("Batch transfer completed: %d success, %d failed", successCount, failedCount)
	if !success {
		message = fmt.Sprintf("Batch transfer completed with errors: %d success, %d failed", successCount, failedCount)
	}

	// 计算总估算费用
	var totalEstimatedFee string
	totalGasUsed := uint64(0)
	for _, tx := range transactions {
		totalGasUsed += tx.GasUsed
	}
	if len(transactions) > 0 {
		// 使用第一个交易的gas价格估算（简化处理）
		totalEstimatedFee = fmt.Sprintf("%d", totalGasUsed)
	}

	return &pb.BatchTransferResp{
		Success:           success,
		Message:           message,
		Transactions:      transactions,
		Total:             int32(len(transactions)),
		TotalEstimatedFee: totalEstimatedFee,
	}, nil
}
