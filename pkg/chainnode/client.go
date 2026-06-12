package chainnode

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	tronClient "github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc"

	"internalwallet/proto/pb"
)

// Client is the direct-node blockchain client used by consolidation.
// It intentionally mirrors the subset of ChainRPC methods that consolidation needs.
type Client interface {
	GetBalance(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error)
	GetTokenBalance(ctx context.Context, in *pb.GetTokenBalanceReq) (*pb.GetTokenBalanceResp, error)
	GetTokenInfo(ctx context.Context, in *pb.GetTokenInfoReq) (*pb.GetTokenInfoResp, error)

	// SuggestGasPrice returns the current suggested EVM gas price in wei (base-10 string).
	// Only ETH/BSC are supported.
	SuggestGasPrice(ctx context.Context, chain pb.ChainRpcType) (string, error)

	EstimateGas(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error)
	BuildTransaction(ctx context.Context, in *pb.BuildTransactionReq) (*pb.BuildTransactionResp, error)
	BroadcastTransaction(ctx context.Context, in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error)
	GetTransactionReceipt(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error)
	GetBlockHeight(ctx context.Context, in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error)

	EstimateTronFee(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error)
	GetTronFeeRates(ctx context.Context) (bandwidthFeeSunPerUnit int64, energyFeeSunPerUnit int64, err error)
	GetTronAccountResources(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error)
	BuildTronTransaction(ctx context.Context, in *pb.ChainRpcBuildTronTransactionReq) (*pb.ChainRpcBuildTronTransactionResp, error)
}

type directClient struct {
	evmMu      sync.RWMutex
	evmClients map[pb.ChainRpcType]*ethclient.Client

	tronMu  sync.RWMutex
	tronCfg *tronNodeConfig
}

type tronNodeConfig struct {
	endpoint         string
	apiKey           string
	trc20FeeLimitSun int64
}

func New(chains []ChainConfig) (Client, error) {
	c := &directClient{
		evmClients: make(map[pb.ChainRpcType]*ethclient.Client),
	}

	for _, cfg := range chains {
		if !cfg.Enabled {
			continue
		}

		switch strings.ToUpper(strings.TrimSpace(cfg.ChainType)) {
		case "ETH":
			cli, err := createETHClientWithTimeout(cfg.RPCEndpoint, cfg.APIKey)
			if err != nil {
				return nil, fmt.Errorf("init ETH client: %w", err)
			}
			c.evmClients[pb.ChainRpcType_CHAIN_TYPE_ETHEREUM] = cli
		case "BSC":
			cli, err := createETHClientWithTimeout(cfg.RPCEndpoint, cfg.APIKey)
			if err != nil {
				return nil, fmt.Errorf("init BSC client: %w", err)
			}
			c.evmClients[pb.ChainRpcType_CHAIN_TYPE_BSC] = cli
		case "TRON":
			if strings.TrimSpace(cfg.TronEndpoint) == "" {
				return nil, fmt.Errorf("init TRON client: TronEndpoint is required")
			}
			feeLimit := cfg.TronTrc20FeeLimitSun
			if feeLimit <= 0 {
				feeLimit = 100_000_000
			}
			c.tronCfg = &tronNodeConfig{
				endpoint:         strings.TrimSpace(cfg.TronEndpoint),
				apiKey:           strings.TrimSpace(cfg.TronAPIKey),
				trc20FeeLimitSun: feeLimit,
			}
		default:
			return nil, fmt.Errorf("unsupported ChainType: %q", cfg.ChainType)
		}
	}

	return c, nil
}

func (c *directClient) getEVM(chain pb.ChainRpcType) (*ethclient.Client, error) {
	c.evmMu.RLock()
	defer c.evmMu.RUnlock()
	cli := c.evmClients[chain]
	if cli == nil {
		return nil, fmt.Errorf("EVM client not configured for chain: %v", chain)
	}
	return cli, nil
}

func (c *directClient) newTron() (*tronClient.GrpcClient, *tronNodeConfig, error) {
	c.tronMu.RLock()
	cfg := c.tronCfg
	c.tronMu.RUnlock()
	if cfg == nil {
		return nil, nil, fmt.Errorf("TRON client not configured")
	}
	grpcClient := tronClient.NewGrpcClientWithTimeout(cfg.endpoint, 15*time.Second)
	if cfg.apiKey != "" {
		if err := grpcClient.SetAPIKey(cfg.apiKey); err != nil {
			return nil, nil, fmt.Errorf("set TRON api key: %w", err)
		}
	}
	if err := grpcClient.Start(grpc.WithInsecure()); err != nil {
		return nil, nil, fmt.Errorf("connect TRON: %w", err)
	}
	return grpcClient, cfg, nil
}

// createETHClientWithTimeout creates an ethclient with reasonable HTTP timeouts and optional API key header.
func createETHClientWithTimeout(endpoint string, apiKey string) (*ethclient.Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("RPCEndpoint is required")
	}

	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		DisableCompression: false,
		ForceAttemptHTTP2:  true,
	}

	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
	if apiKey != "" {
		httpClient.Transport = &apiKeyTransport{
			apiKey:    apiKey,
			headerKey: "X-API-Key",
			inner:     transport,
		}
	}

	rpcClient, err := rpc.DialHTTPWithClient(endpoint, httpClient)
	if err != nil {
		return nil, fmt.Errorf("dial rpc endpoint: %w", err)
	}

	ethClient := ethclient.NewClient(rpcClient)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := ethClient.ChainID(ctx); err != nil {
		rpcClient.Close()
		return nil, fmt.Errorf("verify connection (ChainID): %w", err)
	}

	if apiKey != "" {
		logx.Infof("✓ EVM node connected (apiKey auth): %s", endpoint)
	} else {
		logx.Infof("✓ EVM node connected: %s", endpoint)
	}
	return ethClient, nil
}

type apiKeyTransport struct {
	apiKey    string
	headerKey string
	inner     http.RoundTripper
}

func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if t.apiKey != "" && t.headerKey != "" {
		req.Header.Set(t.headerKey, t.apiKey)
	}
	return t.inner.RoundTrip(req)
}
