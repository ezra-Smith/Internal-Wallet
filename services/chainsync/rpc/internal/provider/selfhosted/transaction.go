package selfhosted

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

func (p *SelfHostedProvider) GetTransaction(ctx context.Context, txHash string) (tx *pb.Transaction, err error) {
	start := time.Now()
	defer p.finishRequest(start, &err)

	if p.config.Chain == pb.BlockChainType_CHAIN_TYPE_TRON {
		resp, err := p.getTronTransaction(ctx, txHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get TRON transaction %s: %v", txHash, err)
		}

		txData := &resp.Transaction.RawData
		txData.TxID = resp.Transaction.TxID
		if len(resp.Transaction.Ret) > 0 {
			txData.Ret = []TronRet{{ContractRet: resp.Transaction.Ret[0].ContractRet}}
		}

		transaction := convertTronTransactionToPb(
			txData,
			p.config.Chain,
			0,
			"",
			txData.Timestamp,
			0,
		)

		txInfo, infoErr := p.getTronTransactionInfo(ctx, txHash)
		if infoErr != nil {
			logx.Debugf("Failed to get transaction info for %s: %v", txHash, infoErr)
			return transaction, nil
		}

		enrichTronTransactionWithLogs(transaction, txInfo)

		if len(txInfo.Log) > 0 {
			transfers := parseTrc20TransfersFromLogs(txInfo.Log)
			if len(transfers) > 0 {
				updateTransactionWithTransferEvents(transaction, transfers)
			}
		}

		return transaction, nil
	}

	return p.getEvmTransaction(ctx, txHash)
}
