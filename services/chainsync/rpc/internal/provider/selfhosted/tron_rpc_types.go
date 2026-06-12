package selfhosted

// TronRPCTransactionInfo is the TRON tx detail response for self-hosted provider.
type TronRPCTransactionInfo struct {
	ID             string         `json:"id"`
	Fee            int64          `json:"fee"`
	BlockNumber    int64          `json:"blockNumber"`
	BlockTimestamp int64          `json:"blockTimeStamp"`
	ContractResult []string       `json:"contractResult"`
	Receipt        TronRPCReceipt `json:"receipt"`
	Log            []*TronEventLog `json:"log"`
	Result         string         `json:"result"` // SUCCESS or REVERT
	ResMessage     string         `json:"resMessage"`
}

type TronRPCReceipt struct {
	EnergyUsage      int64  `json:"energy_usage"`
	EnergyUsageTotal int64  `json:"energy_usage_total"`
	NetUsage         int64  `json:"net_usage"`
	Result           string `json:"result"`
}

type TronEventLog struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

