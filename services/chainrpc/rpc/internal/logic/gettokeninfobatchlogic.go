package logic

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetTokenInfoBatchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTokenInfoBatchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTokenInfoBatchLogic {
	return &GetTokenInfoBatchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// isValidTronAddress 验证TRON地址格式
func (l *GetTokenInfoBatchLogic) isValidTronAddress(addr string) bool {
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

// validateGetTokenInfoBatchReq 验证批量查询请求参数
func (l *GetTokenInfoBatchLogic) validateGetTokenInfoBatchReq(in *pb.GetTokenInfoBatchReq) error {
	if in == nil {
		return fmt.Errorf("request is nil")
	}

	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return fmt.Errorf("chain type is required")
	}

	// 检查代币合约地址列表
	if len(in.TokenContracts) == 0 {
		return fmt.Errorf("token contracts list cannot be empty")
	}

	// 验证每个代币合约地址
	for i, contract := range in.TokenContracts {
		if contract == "" {
			return fmt.Errorf("token contract at index %d cannot be empty", i)
		}

		// 根据链类型验证地址格式
		switch in.Chain {
		case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
			if !common.IsHexAddress(contract) {
				return fmt.Errorf("invalid token contract address format at index %d: %s", i, contract)
			}
		case pb.ChainRpcType_CHAIN_TYPE_TRON:
			if !l.isValidTronAddress(contract) {
				return fmt.Errorf("invalid TRON token contract address at index %d: %s", i, contract)
			}
		default:
			return fmt.Errorf("unsupported chain type: %v", in.Chain)
		}
	}

	return nil
}

// GetTokenInfoBatch 批量获取代币信息
func (l *GetTokenInfoBatchLogic) GetTokenInfoBatch(in *pb.GetTokenInfoBatchReq) (*pb.GetTokenInfoBatchResp, error) {
	// 参数验证
	if err := l.validateGetTokenInfoBatchReq(in); err != nil {
		return &pb.GetTokenInfoBatchResp{
			Success: false,
			Message: fmt.Sprintf("validation failed: %v", err),
			Tokens:  []*pb.TokenInfo{},
			Total:   0,
		}, nil
	}

	// 检查代币合约地址列表
	if len(in.TokenContracts) == 0 {
		return &pb.GetTokenInfoBatchResp{
			Success: false,
			Message: "token contracts list is empty",
			Tokens:  []*pb.TokenInfo{},
			Total:   0,
		}, nil
	}

	// 记录开始时间
	startTime := time.Now()
	l.Logger.Infof("Starting batch token info query for %d contracts", len(in.TokenContracts))

	// 初始化结果切片
	tokens := make([]*pb.TokenInfo, 0, len(in.TokenContracts))
	successCount := 0
	errorCount := 0

	// 并发查询代币信息（限制并发数以避免过载）
	const maxConcurrency = 10
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 创建单个查询逻辑实例
	singleLogic := NewGetTokenInfoLogic(l.ctx, l.svcCtx)

	// 为每个代币合约创建查询任务
	for i, contractAddress := range in.TokenContracts {
		wg.Add(1)
		go func(index int, address string) {
			defer wg.Done()
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			// 创建单个查询请求
			singleReq := &pb.GetTokenInfoReq{
				Chain:         in.Chain,
				TokenContract: address,
			}

			// 调用单个代币信息查询函数
			l.Logger.Debugf("Querying token %d/%d: %s", index+1, len(in.TokenContracts), address)
			resp, err := singleLogic.GetTokenInfo(singleReq)

			mu.Lock()
			if err != nil {
				// 查询失败，创建错误信息的TokenInfo
				errorToken := &pb.TokenInfo{
					ContractAddress:      address,
					Name:                 "Unknown",
					Symbol:               "ERROR",
					Decimals:             0,
					TotalSupply:          "0",
					TotalSupplyFormatted: "0",
					Owner:                "",
					Verified:             false,
					PriceUsd:             0.0,
				}
				tokens = append(tokens, errorToken)
				errorCount++
				l.Logger.Errorf("Failed to query token %s: %v", address, err)
			} else if resp.Success {
				// 查询成功，添加到结果中
				tokens = append(tokens, resp.TokenInfo)
				successCount++
				l.Logger.Infof("Successfully queried token %s: %s", address, resp.TokenInfo.Symbol)
			} else {
				// 查询失败但返回了响应
				errorToken := &pb.TokenInfo{
					ContractAddress:      address,
					Name:                 "Unknown",
					Symbol:               "ERROR",
					Decimals:             0,
					TotalSupply:          "0",
					TotalSupplyFormatted: "0",
					Owner:                "",
					Verified:             false,
					PriceUsd:             0.0,
				}
				tokens = append(tokens, errorToken)
				errorCount++
				l.Logger.Errorf("Token query failed for %s: %s", address, resp.Message)
			}
			mu.Unlock()
		}(i, contractAddress)
	}

	// 等待所有查询完成
	wg.Wait()

	// 记录结束时间
	duration := time.Since(startTime)
	l.Logger.Infof("Batch token info query completed in %v: %d success, %d failed",
		duration, successCount, errorCount)

	// 构建响应
	success := errorCount == 0
	message := fmt.Sprintf("Batch query completed: %d success, %d failed", successCount, errorCount)
	if !success {
		message = fmt.Sprintf("Batch query completed with errors: %d success, %d failed", successCount, errorCount)
	}

	return &pb.GetTokenInfoBatchResp{
		Success: success,
		Message: message,
		Tokens:  tokens,
		Total:   int32(len(tokens)),
	}, nil
}
