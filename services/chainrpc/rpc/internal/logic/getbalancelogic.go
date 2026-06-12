package logic

import (
	"context"
	"fmt"
	"strings"
	
	"github.com/ethereum/go-ethereum/common"
	"github.com/zeromicro/go-zero/core/logx"
	"math/big"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type GetBalanceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBalanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBalanceLogic {
	return &GetBalanceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetBalance 查询地址余额
func (l *GetBalanceLogic) GetBalance(in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return l.getETHBalance(in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return l.getTronBalance(in)
	default:
		return nil, fmt.Errorf("unsupported chain type: %v", in.Chain)
	}
}

// getETHBalance 查询 ETH/BSC 地址余额
func (l *GetBalanceLogic) getETHBalance(in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	// Get ETH client
	client, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get ETH client: %v", err)
	}

	// Parse address
	address := common.HexToAddress(in.Address)

	// Get balance
	var balance *big.Int
	var blockNumber uint64
	if in.BlockNumber != 0 {
		balance, err = client.BalanceAt(l.ctx, address, big.NewInt(int64(in.BlockNumber)))
	} else {
		// Query latest balance
		balance, err = client.BalanceAt(l.ctx, address, nil)
		blockNumber, err = client.BlockNumber(l.ctx)
		if err != nil {
			fmt.Errorf("getETHBalance() failed to get latest block number: %v", err)
		}
	}

	if err != nil {
		return &pb.GetBalanceResp{
			Success:          false,
			Message:          "Balance retrieved failed",
			Balance:          "0",
			BalanceFormatted: "0",
			BlockNumber:      blockNumber,
		}, nil
	}

	// Convert to ETH format (divide by 10^18)
	ethBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
	formattedBalance := ethBalance.String()

	return &pb.GetBalanceResp{
		Success:          true,
		Message:          "Balance retrieved successfully",
		Balance:          balance.String(),
		BalanceFormatted: formattedBalance,
		BlockNumber:      blockNumber,
	}, nil
}

// getTronBalance 查询 TRON 地址余额
func (l *GetBalanceLogic) getTronBalance(in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	// Get TRON client
	tronClient, err := l.svcCtx.ChainMgr.GetTronClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON client: %v", err)
	}
	defer tronClient.Stop()

	// Convert address to hex format if needed
	address := in.Address
	if len(address) != 42 || address[:2] != "41" {
		// Assume base58 format, convert to hex
		// TODO: Implement proper base58 to hex conversion
		address = address // For now, use as-is
	}

	// Get account
	account, err := tronClient.GetAccount(address)
	if err != nil {
		// TRON 账户未激活时会返回错误，这是正常情况，余额为 0
		// 但也可能是其他错误（网络、API限流等），需要记录详细错误
		l.Errorf("TRON GetAccount failed: address=%s, error=%v", in.Address, err)
		// 仅当明确是账户不存在错误时才视为未激活
		if strings.Contains(err.Error(), "account not found") || strings.Contains(err.Error(), "Account does not exist") {
			l.Infof("TRON account not found (not activated): address=%s, treating as zero balance", in.Address)
			account = nil
		} else {
			// 其他错误直接返回
		return nil, fmt.Errorf("failed to get TRON account: %v", err)
		}
	}

	var balance *big.Int
	if account != nil && account.Balance > 0 {
		balance = big.NewInt(account.Balance)
	} else {
		balance = big.NewInt(0)
	}

	// Convert to TRX format (divide by 10^6)
	trxBalance := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e6))
	formattedBalance := trxBalance.String()

	block, err := tronClient.GetNowBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get TRON block: %v", err)
	}

	return &pb.GetBalanceResp{
		Success:          true,
		Message:          "TRON balance retrieved successfully",
		Balance:          balance.String(),
		BalanceFormatted: formattedBalance,
		BlockNumber:      uint64(block.BlockHeader.RawData.Number),
	}, nil
}
