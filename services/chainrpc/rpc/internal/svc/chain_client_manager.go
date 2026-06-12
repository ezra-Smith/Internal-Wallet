package svc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/fbsobreira/gotron-sdk/pkg/client"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/config"
)

// ChainClientManager manages blockchain clients for different chains
type ChainClientManager struct {
	config          config.Config
	clients         map[pb.ChainRpcType]interface{}
	mu              sync.RWMutex
	signer          pb.SignerServiceClient
	signerRpcClient zrpc.Client
}

// NewChainClientManager creates a new chain client manager
func NewChainClientManager(c config.Config) (*ChainClientManager, error) {
	manager := &ChainClientManager{
		config:  c,
		clients: make(map[pb.ChainRpcType]interface{}),
	}

	// Initialize signer client using etcd service discovery
	if err := manager.initSignerClientWithEtcd(); err != nil {
		logx.Errorf("Failed to initialize signer client with etcd: %v", err)
		return nil, err
	}

	// Initialize chain clients
	if err := manager.initChainClients(); err != nil {
		logx.Errorf("Failed to initialize chain clients: %v", err)
		return nil, err
	}

	return manager, nil
}

// initSignerClientWithEtcd initializes the signer service client using etcd service discovery
func (m *ChainClientManager) initSignerClientWithEtcd() error {
	// Check if SignerRpc is configured (has etcd or endpoints)
	if len(m.config.SignerRpc.Endpoints) == 0 && m.config.SignerRpc.Etcd.Key == "" {
		// Fallback to direct connection for backward compatibility
		return m.initSignerClientDirect()
	}

	// Create RPC client with service discovery (etcd or direct endpoints)
	signerRpcClient := zrpc.MustNewClient(m.config.SignerRpc)
	m.signerRpcClient = signerRpcClient
	m.signer = pb.NewSignerServiceClient(signerRpcClient.Conn())

	if m.config.SignerRpc.Etcd.Key != "" {
		logx.Infof("Connected to signer service via etcd discovery with key: %s", m.config.SignerRpc.Etcd.Key)
	} else {
		logx.Infof("Connected to signer service via direct endpoints: %v", m.config.SignerRpc.Endpoints)
	}
	return nil
}

// initSignerClientDirect initializes the signer service client with direct connection (fallback)
func (m *ChainClientManager) initSignerClientDirect() error {
	if m.config.Signer.Host == "" || m.config.Signer.Port == 0 {
		return fmt.Errorf("signer configuration missing and no etcd configuration provided")
	}

	signerAddr := fmt.Sprintf("%s:%d", m.config.Signer.Host, m.config.Signer.Port)

	conn, err := grpc.Dial(signerAddr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to signer service: %v", err)
	}

	m.signer = pb.NewSignerServiceClient(conn)
	logx.Infof("Connected to signer service at %s (direct connection)", signerAddr)
	return nil
}

// initChainClients initializes clients for all configured chains
func (m *ChainClientManager) initChainClients() error {
	for _, chainConfig := range m.config.Chains {
		if !chainConfig.Enabled {
			continue
		}

		var chainType pb.ChainRpcType
		switch chainConfig.ChainType {
		case "ETH":
			chainType = pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
		case "BSC":
			chainType = pb.ChainRpcType_CHAIN_TYPE_BSC
		case "TRON":
			chainType = pb.ChainRpcType_CHAIN_TYPE_TRON
		default:
			logx.Errorf("Unsupported chain type: %s", chainConfig.ChainType)
			continue
		}

		if err := m.initChainClient(chainType, chainConfig); err != nil {
			logx.Errorf("Failed to initialize client for %s: %v", chainConfig.ChainType, err)
			continue
		}
	}

	return nil
}

// initChainClient initializes a single chain client
func (m *ChainClientManager) initChainClient(chainType pb.ChainRpcType, chainConfig config.ChainConfig) error {
	switch chainType {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		if chainConfig.RPCEndpoint == "" {
			return fmt.Errorf("RPC endpoint required for %s", chainConfig.ChainType)
		}

		// 创建带超时配置和 API Key 支持的自定义 HTTP 客户端
		ethClient, err := createETHClientWithTimeout(chainConfig.RPCEndpoint, chainConfig.APIKey)
		if err != nil {
			return fmt.Errorf("failed to connect to %s RPC: %v", chainConfig.ChainType, err)
		}

		m.mu.Lock()
		m.clients[chainType] = ethClient
		m.mu.Unlock()

		if chainConfig.APIKey != "" {
			logx.Infof("Connected to %s RPC at %s with API Key auth (ChainID: %d)", chainConfig.ChainType, chainConfig.RPCEndpoint, chainConfig.ChainID)
		} else {
			logx.Infof("Connected to %s RPC at %s (ChainID: %d)", chainConfig.ChainType, chainConfig.RPCEndpoint, chainConfig.ChainID)
		}

	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		if chainConfig.TronEndpoint == "" {
			return fmt.Errorf("Tron endpoint required for TRON")
		}

		// For TRON, we'll create clients on-demand since gotron-sdk doesn't maintain persistent connections well
		m.mu.Lock()
		m.clients[chainType] = &tronClientWrapper{
			endpoint: chainConfig.TronEndpoint,
			apiKey:   chainConfig.TronAPIKey,
			chainID:  chainConfig.ChainID,
		}
		m.mu.Unlock()

		if chainConfig.TronAPIKey != "" {
			logx.Infof("Configured TRON client for endpoint %s with API Key auth (ChainID: %d)", chainConfig.TronEndpoint, chainConfig.ChainID)
		} else {
			logx.Infof("Configured TRON client for endpoint %s (ChainID: %d)", chainConfig.TronEndpoint, chainConfig.ChainID)
		}

	default:
		return fmt.Errorf("unsupported chain type: %v", chainType)
	}

	return nil
}

// GetETHClient returns the Ethereum client for ETH/BSC chains
func (m *ChainClientManager) GetETHClient(chainType pb.ChainRpcType) (*ethclient.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[chainType]
	if !ok {
		return nil, fmt.Errorf("client not initialized for chain type: %v", chainType)
	}

	ethClient, ok := client.(*ethclient.Client)
	if !ok {
		return nil, fmt.Errorf("client is not an Ethereum client for chain type: %v", chainType)
	}

	return ethClient, nil
}

// GetTronClient returns a new TRON client instance
func (m *ChainClientManager) GetTronClient() (*client.GrpcClient, error) {
	m.mu.RLock()
	wrapper, ok := m.clients[pb.ChainRpcType_CHAIN_TYPE_TRON]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("TRON client not configured")
	}

	tronWrapper, ok := wrapper.(*tronClientWrapper)
	if !ok {
		return nil, fmt.Errorf("invalid TRON client wrapper")
	}

	return tronWrapper.createClient()
}

// GetSignerClient returns the signer service client
func (m *ChainClientManager) GetSignerClient() pb.SignerServiceClient {
	return m.signer
}

// Close closes all clients
func (m *ChainClientManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Note: zrpc.Client doesn't need explicit closing, it's managed internally

	// Close blockchain clients
	for chainType, client := range m.clients {
		switch chainType {
		case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
			if ethClient, ok := client.(*ethclient.Client); ok {
				ethClient.Close()
			}
		}
	}

	logx.Info("All chain clients closed")
}

// tronClientWrapper creates TRON clients on-demand
type tronClientWrapper struct {
	endpoint string
	apiKey   string
	chainID  int64
}

func (w *tronClientWrapper) createClient() (*client.GrpcClient, error) {
	grpcClient := client.NewGrpcClientWithTimeout(w.endpoint, 15*time.Second)

	// 如果配置了 API Key,则设置 API Key
	if w.apiKey != "" {
		if err := grpcClient.SetAPIKey(w.apiKey); err != nil {
			return nil, fmt.Errorf("failed to set TRON API key: %v", err)
		}
	}

	if err := grpcClient.Start(grpc.WithInsecure()); err != nil {
		return nil, fmt.Errorf("failed to connect to TRON: %v", err)
	}

	return grpcClient, nil
}

// createETHClientWithTimeout 创建带超时配置和 API Key 支持的 Ethereum 客户端
// 解决默认 ethclient.Dial() 没有 HTTP 超时配置导致的偶发超时问题
// endpoint: RPC 节点地址
// apiKey: 可选的 API Key,用于节点认证
func createETHClientWithTimeout(endpoint string, apiKey string) (*ethclient.Client, error) {
	// 创建自定义的 HTTP Transport，配置各种超时参数
	transport := &http.Transport{
		// 连接池配置
		MaxIdleConns:        10,               // 最大空闲连接数
		MaxIdleConnsPerHost: 5,                // 每个主机的最大空闲连接数
		IdleConnTimeout:     90 * time.Second, // 空闲连接超时

		// 连接超时配置
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // TCP 连接建立超时（包括 DNS 解析）
			KeepAlive: 30 * time.Second, // TCP keep-alive 探测间隔
		}).DialContext,

		// 响应超时配置
		ResponseHeaderTimeout: 15 * time.Second, // 等待响应头的超时时间
		ExpectContinueTimeout: 1 * time.Second,  // 100-continue 响应超时

		// TLS 配置
		TLSHandshakeTimeout: 10 * time.Second, // TLS 握手超时
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12, // 最低 TLS 1.2
		},

		// 性能优化
		DisableCompression: false, // 启用压缩
		ForceAttemptHTTP2:  true,  // 尝试使用 HTTP/2
	}

	// 创建自定义 HTTP 客户端（总超时时间）
	httpClient := &http.Client{
		Timeout:   30 * time.Second, // HTTP 请求总超时（包括连接、读取、写入）
		Transport: transport,
	}

	// 如果提供了 API Key,则包装 Transport 以添加认证头
	if apiKey != "" {
		httpClient.Transport = &apiKeyTransport{
			apiKey:    apiKey,
			headerKey: "X-API-Key", // 自建节点要求的头名称
			inner:     transport,
		}
	}

	// 使用自定义 HTTP 客户端创建 RPC 客户端
	rpcClient, err := rpc.DialHTTPWithClient(endpoint, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to dial RPC endpoint: %w", err)
	}

	// 验证连接（带超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ethClient := ethclient.NewClient(rpcClient)

	// 快速健康检查：尝试获取链 ID
	if _, err := ethClient.ChainID(ctx); err != nil {
		rpcClient.Close()
		return nil, fmt.Errorf("failed to verify connection (ChainID check): %w", err)
	}

	if apiKey != "" {
		logx.Infof("✅ ETH client created successfully with API Key auth and timeout configurations: endpoint=%s", endpoint)
	} else {
		logx.Infof("✅ ETH client created successfully with timeout configurations: endpoint=%s", endpoint)
	}

	return ethClient, nil
}

// apiKeyTransport 是一个自定义的 HTTP RoundTripper,用于在每个请求中添加 API Key
type apiKeyTransport struct {
	apiKey    string            // API Key 值
	headerKey string            // HTTP 头名称
	inner     http.RoundTripper // 底层 Transport
}

// RoundTrip 实现 http.RoundTripper 接口
func (t *apiKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 克隆请求以避免修改原始请求
	req = req.Clone(req.Context())

	// 添加 API Key 到请求头
	if t.apiKey != "" && t.headerKey != "" {
		req.Header.Set(t.headerKey, t.apiKey)
	}

	// 使用底层 Transport 发送请求
	return t.inner.RoundTrip(req)
}
