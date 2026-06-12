package models

import "internalwallet/common/model"

// SignatureLog 签名日志模型
type SignatureLog struct {
	model.BaseModel
	RequestID      string  `gorm:"column:request_id" json:"request_id"`
	MasterSeedID   int64   `gorm:"column:master_seed_id" json:"master_seed_id"`
	Chain          string  `gorm:"column:chain" json:"chain"`
	DerivationPath string  `gorm:"column:derivation_path" json:"derivation_path"`
	FromAddress    *string `gorm:"column:from_address" json:"from_address,omitempty"`
	OperationType  int     `gorm:"column:operation_type" json:"operation_type"`
	RawTxHash      *string `gorm:"column:raw_tx_hash" json:"raw_tx_hash,omitempty"`
	SignedTx       *string `gorm:"column:signed_tx;type:text" json:"signed_tx,omitempty"`
	TxHash         *string `gorm:"column:tx_hash" json:"tx_hash,omitempty"`
	Amount         *string `gorm:"column:amount" json:"amount,omitempty"`
	ToAddress      *string `gorm:"column:to_address" json:"to_address,omitempty"`
	AssetSymbol    *string `gorm:"column:asset_symbol" json:"asset_symbol,omitempty"`
	TokenContract  *string `gorm:"column:token_contract" json:"token_contract,omitempty"`
	Requester      *string `gorm:"column:requester" json:"requester,omitempty"`
	RequestIP      *string `gorm:"column:request_ip" json:"request_ip,omitempty"`
	Status         int     `gorm:"column:status" json:"status"`
	ErrorMsg       *string `gorm:"column:error_msg" json:"error_msg,omitempty"`
}

// TableName 指定表名
func (SignatureLog) TableName() string {
	return "signature_logs"
}
