package selfhosted

// TronAPIResponse is the common TRON API response shape.
type TronAPIResponse struct {
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

type TronNowBlockResponse struct {
	BlockID      string            `json:"blockID"`
	BlockHeader  TronBlockHeader   `json:"block_header"`
	Transactions []TronTransaction `json:"transactions,omitempty"`
}

type TronBlockByNumResponse struct {
	BlockID      string            `json:"blockID"`
	BlockHeader  TronBlockHeader   `json:"block_header"`
	Transactions []TronTransaction `json:"transactions,omitempty"`
}

type TronTransactionResponse struct {
	Visible     bool            `json:"visible"`
	Ret         []interface{}   `json:"ret"`
	Transaction TronTransaction `json:"transaction"`
}

type TronAccountResponse struct {
	Address string `json:"address"`
	Balance int64  `json:"balance"`
}

type TronBlockInfo struct {
	Hash      string `json:"hash"`
	Number    int64  `json:"number"`
	Timestamp string `json:"timestamp"`
}

type TronBlockHeader struct {
	RawData TronBlockData `json:"raw_data"`
}

type TronBlockData struct {
	Number         int64  `json:"number"`
	Timestamp      int64  `json:"timestamp"`
	TxTrieRoot     string `json:"txTrieRoot"`
	WitnessAddress string `json:"witness_address"`
	ParentHash     string `json:"parentHash"`
}

type TronTransaction struct {
	TxID    string             `json:"txID"`
	RawData TronTransactionData `json:"raw_data"`
	Ret     []TronTransactionRet `json:"ret"`
}

type TronTransactionRet struct {
	ContractRet string `json:"contractRet"`
	Ret         int    `json:"ret"`
}

type TronTransactionData struct {
	TxID          string         `json:"txID"`
	Contract      []TronContract `json:"contract"`
	RefBlockBytes string         `json:"ref_block_bytes"`
	RefBlockHash  string         `json:"ref_block_hash"`
	Expiration    int64          `json:"expiration"`
	Timestamp     int64          `json:"timestamp"`
	Ret           []TronRet      `json:"ret"`
}

type TronRet struct {
	ContractRet string `json:"contractRet"`
}

type TronContract struct {
	Parameter TronParameter `json:"parameter"`
	Type      string        `json:"type"`
}

type TronParameter struct {
	Value TronValue `json:"value"`
}

type TronValue struct {
	Amount          int64  `json:"amount,omitempty"`
	OwnerAddress    string `json:"owner_address,omitempty"`
	ToAddress       string `json:"to_address,omitempty"`
	ContractAddress string `json:"contract_address,omitempty"`
	Data            string `json:"data,omitempty"`
	AssetName       string `json:"asset_name,omitempty"`
	CallValue       int64  `json:"call_value,omitempty"`
}
