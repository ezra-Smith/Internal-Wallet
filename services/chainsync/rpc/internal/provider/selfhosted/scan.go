package selfhosted

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
	tronutil "internalwallet/services/chainsync/rpc/internal/provider/tron"
)

func (p *SelfHostedProvider) scanBlockForAddress(block *pb.ChainBlockInfo, address string) []*pb.Transaction {
	var transactions []*pb.Transaction
	ctx := context.Background()

	logx.Infof("Scanning block %d for address %s on chain %v", block.BlockNumber, address, p.config.Chain)

	switch p.config.Chain {
	case pb.BlockChainType_CHAIN_TYPE_ETHEREUM, pb.BlockChainType_CHAIN_TYPE_BSC:
		transactions = p.scanEthereumBlockForAddress(ctx, block, address)
	case pb.BlockChainType_CHAIN_TYPE_TRON:
		transactions = p.scanTronBlockForAddress(ctx, block, address)
	default:
		logx.Errorf("Unsupported chain type: %v", p.config.Chain)
		return nil
	}

	logx.Infof("Found %d transactions for address %s in block %d", len(transactions), address, block.BlockNumber)
	return transactions
}

func (p *SelfHostedProvider) scanEthereumBlockForAddress(ctx context.Context, block *pb.ChainBlockInfo, address string) []*pb.Transaction {
	var transactions []*pb.Transaction

	blockNumberHex := fmt.Sprintf("0x%x", block.BlockNumber)
	resp, err := p.callRPC(ctx, "eth_getBlockByNumber", []interface{}{blockNumberHex, true})
	if err != nil {
		logx.Errorf("Failed to get block details for scanning: %v", err)
		return nil
	}

	var blockData map[string]interface{}
	if err := json.Unmarshal(resp.Result, &blockData); err != nil {
		logx.Errorf("Failed to unmarshal block data: %v", err)
		return nil
	}

	txs, ok := blockData["transactions"].([]interface{})
	if !ok {
		logx.Debugf("No transactions found in block %d", block.BlockNumber)
		return nil
	}

	targetAddress := address
	for i, txInterface := range txs {
		tx, ok := txInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if p.isEthereumTransactionRelevant(tx, targetAddress) {
			pbTx := p.parseEthereumTransaction(tx, uint32(i), block)
			if pbTx != nil {
				transactions = append(transactions, pbTx)
			}
		}
	}

	return transactions
}

func (p *SelfHostedProvider) scanTronBlockForAddress(ctx context.Context, block *pb.ChainBlockInfo, address string) []*pb.Transaction {
	var transactions []*pb.Transaction

	blockResp, err := p.getTronBlockByNumber(ctx, block.BlockNumber)
	if err != nil {
		logx.Errorf("Failed to get TRON block details for scanning: %v", err)
		return nil
	}

	logx.Infof("📦 [%s] Block %d contains %d transactions (hash: %s)",
		p.config.Type, blockResp.BlockHeader.RawData.Number, len(blockResp.Transactions), blockResp.BlockID)

	for i, tronTx := range blockResp.Transactions {
		if p.isTronTransactionRelevant(tronTx, address) {
			pbTx := p.parseTronTransaction(&tronTx, block, uint32(i))
			if pbTx != nil {
				transactions = append(transactions, pbTx)
			}
		}
	}

	return transactions
}

func (p *SelfHostedProvider) isEthereumTransactionRelevant(tx map[string]interface{}, targetAddress string) bool {
	from, ok := tx["from"].(string)
	if !ok {
		return false
	}
	to, hasTo := tx["to"].(string)

	if from == targetAddress {
		return true
	}
	if hasTo && to != "" && to == targetAddress {
		return true
	}

	return false
}

func (p *SelfHostedProvider) isTronTransactionRelevant(tx TronTransaction, targetAddress string) bool {
	for _, contract := range tx.RawData.Contract {
		fromAddr := tronutil.HexToBase58(contract.Parameter.Value.OwnerAddress)
		toAddr := tronutil.HexToBase58(contract.Parameter.Value.ToAddress)
		if fromAddr == targetAddress || toAddr == targetAddress {
			return true
		}
	}
	return false
}
