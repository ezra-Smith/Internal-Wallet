package token

import "internalwallet/proto/pb"

// Info describes an on-chain token contract.
type Info struct {
	Address      string `json:"address"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Decimals     uint8  `json:"decimals"`
	Chain        string `json:"chain"`
	IsStablecoin bool   `json:"is_stablecoin"`
}

// Transaction is a normalized token transfer extracted from a chain tx.
type Transaction struct {
	TxHash      string            `json:"tx_hash"`
	Chain       pb.BlockChainType `json:"chain"`
	BlockNumber uint64            `json:"block_number"`
	BlockHash   string            `json:"block_hash"`
	From        string            `json:"from"`
	To          string            `json:"to"`
	Status      uint8             `json:"status"`
	Timestamp   int64             `json:"timestamp"`

	TokenAddress    string `json:"token_address"`
	TokenName       string `json:"token_name"`
	TokenSymbol     string `json:"token_symbol"`
	TokenDecimals   uint8  `json:"token_decimals"`
	TokenAmount     string `json:"token_amount"`
	TokenValue      string `json:"token_value"`
	TransactionType string `json:"transaction_type"` // "native" or "token"
}

