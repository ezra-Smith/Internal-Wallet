package chainnode

// ChainConfig defines a direct-node connection for a single chain.
//
// NOTE: This config is intended to be embedded at the top-level service config:
//
//	Chains:
//	  - ChainType: "ETH"
//	    RPCEndpoint: "https://..."
//	  - ChainType: "TRON"
//	    TronEndpoint: "grpc.trongrid.io:50051"
type ChainConfig struct {
	ChainType string `json:",default=ETH"` // ETH, BSC, TRON
	ChainID   int64  `json:",optional"`    // EVM chain ID (optional)

	// ETH/BSC
	RPCEndpoint string `json:",optional"` // HTTP RPC endpoint
	APIKey      string `json:",optional"` // optional node auth header

	// TRON
	TronEndpoint string `json:",optional"` // gRPC endpoint
	TronAPIKey   string `json:",optional"` // optional node auth

	// TRC20 feeLimit for BuildTronTransaction (SUN). Default: 100 TRX.
	TronTrc20FeeLimitSun int64 `json:",optional,default=100000000"`

	// Enabled controls whether this chain client is initialized.
	Enabled bool `json:",default=true"`
}
