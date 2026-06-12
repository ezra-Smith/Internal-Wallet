package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetWeb3TokenDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetWeb3TokenDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetWeb3TokenDetailLogic {
	return &GetWeb3TokenDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetWeb3TokenDetail 获取 Web3 代币详细信息（公开查询，无需认证）
func (l *GetWeb3TokenDetailLogic) GetWeb3TokenDetail(in *pb.GetWeb3TokenDetailReq) (*pb.GetWeb3TokenDetailResp, error) {
	// 验证参数
	assetCode := strings.ToUpper(strings.TrimSpace(in.AssetCode))
	network := strings.ToUpper(strings.TrimSpace(in.Network))
	contractAddress := strings.TrimSpace(in.ContractAddress)

	if assetCode == "" {
		return nil, errx.Web3AssetCodeRequired()
	}

	if network == "" {
		return nil, errx.Web3NetworkRequired()
	}

	// 将网络名称转换为 ChainRpcType
	chainType, err := l.networkToChainRpcType(network)
	if err != nil {
		return nil, errx.Web3UnsupportedNetwork(network)
	}

	// 如果没有提供合约地址，尝试从数据库查询（仅对代币）
	if contractAddress == "" && !l.isNativeToken(assetCode, network) {
		chainCode := l.networkToChainCode(network)
		if chainCode != "" && l.svcCtx.CurrencyChainSettingsRepository != nil {
			setting, err := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(l.ctx, assetCode, chainCode)
			if err == nil && setting != nil && setting.ContractAddress != nil && *setting.ContractAddress != "" {
				contractAddress = *setting.ContractAddress
				l.Infof("从数据库查询到合约地址: asset=%s, network=%s, contract=%s", assetCode, network, contractAddress)
			}
		}
	}

	// 初始化响应
	resp := &pb.GetWeb3TokenDetailResp{
		Success:              true,
		Message:              "ok",
		Symbol:               assetCode,
		ContractAddress:      "~", // 默认值，主币显示 "~"
		Network:              l.getNetworkDisplayName(network),
		MintTime:             "~",
		IssuerAddress:        "~",
		Holders:              "~",
		TotalSupply:          "0",
		CirculatingSupply:    "0",
		TotalMarketCap:       "0",
		CirculatingMarketCap: "0",
		Decimals:             18, // 默认精度
	}

	// 设置合约地址（主币为空或 "~"，代币为实际地址）
	if contractAddress != "" {
		resp.ContractAddress = contractAddress
	}

	// 获取链ID
	resp.ChainId = l.getChainId(network)

	// 从 accounting 服务获取资产图标和名称
	if l.svcCtx.AccountingRpc != nil {
		assetResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
		if err == nil && assetResp != nil && assetResp.Success && assetResp.Item != nil {
			resp.IconUrl = assetResp.Item.IconUrl
			if assetResp.Item.Name != "" {
				resp.Name = assetResp.Item.Name
			} else {
				resp.Name = assetCode
			}
		} else {
			resp.Name = assetCode
		}
	} else {
		resp.Name = assetCode
	}

	// 从 chainrpc 服务获取代币基本信息（合约地址、名称、符号、精度、总供应量、所有者）
	if l.svcCtx.ChainRpc != nil && contractAddress != "" {
		tokenInfoResp, err := l.svcCtx.ChainRpc.GetTokenInfo(l.ctx, &pb.GetTokenInfoReq{
			Chain:         chainType,
			TokenContract: contractAddress,
		})
		if err == nil && tokenInfoResp != nil && tokenInfoResp.Success && tokenInfoResp.TokenInfo != nil {
			tokenInfo := tokenInfoResp.TokenInfo

			// 更新代币信息
			if tokenInfo.Name != "" {
				resp.Name = tokenInfo.Name
			}
			if tokenInfo.Symbol != "" {
				resp.Symbol = tokenInfo.Symbol
			}
			if tokenInfo.Decimals > 0 {
				resp.Decimals = tokenInfo.Decimals
			}
			if tokenInfo.TotalSupplyFormatted != "" {
				resp.TotalSupply = tokenInfo.TotalSupplyFormatted
			} else if tokenInfo.TotalSupply != "" {
				resp.TotalSupply = tokenInfo.TotalSupply
			}

			// 对于代币，流通供应量通常等于总供应量（除非有锁定机制）
			// 先使用总供应量作为流通供应量，然后尝试从第三方API查询更准确的流通供应量
			if resp.TotalSupply != "" && resp.TotalSupply != "0" && resp.TotalSupply != "~" {
				resp.CirculatingSupply = resp.TotalSupply
				// 尝试从第三方API查询更准确的流通供应量（异步，不阻塞主流程）
				go func() {
					circulatingSupply := l.queryTokenCirculatingSupply(network, contractAddress, assetCode)
					if circulatingSupply != "" && circulatingSupply != resp.TotalSupply {
						// 如果查询到的流通供应量与总供应量不同，更新缓存供下次使用
						if l.svcCtx.RedisClient != nil {
							cacheKey := fmt.Sprintf("token:supply:%s:%s:circulating", network, contractAddress)
							l.svcCtx.RedisClient.Set(context.Background(), cacheKey, circulatingSupply, 1*time.Hour)
						}
					}
				}()
			}

			// 设置发行地址（所有者）
			if tokenInfo.Owner != "" {
				resp.IssuerAddress = tokenInfo.Owner
			}

			// 使用 chainrpc 返回的价格（如果有）
			if tokenInfo.PriceUsd > 0 {
				resp.PriceUsd = fmt.Sprintf("%.4f", tokenInfo.PriceUsd)
			}
		}
	}

	// 对于主币，从链上查询总供应量和流通供应量
	if contractAddress == "" && l.isNativeToken(assetCode, network) {
		l.queryNativeTokenSupply(assetCode, network, chainType, resp)
	}

	// 从 market 服务（Redis）获取价格和24小时涨跌幅
	if l.svcCtx.RedisClient != nil {
		// 获取当前价格（USD价格，而不是USDT价格）
		if price, ok := GetAssetPriceUSD(l.ctx, l.svcCtx.RedisClient, assetCode); ok {
			if resp.PriceUsd == "" || resp.PriceUsd == "0" {
				resp.PriceUsd = price.String()
			}
		}

		// 获取24小时涨跌幅
		hourlyPoints, err := GetHourlySparkline(l.ctx, l.svcCtx.RedisClient, assetCode, 24)
		if err == nil && len(hourlyPoints) > 0 {
			_, changePercent := GenerateSVGSparkline(hourlyPoints, 60, 20)
			resp.PriceChange_24H = changePercent
		}
	}

	// 计算市值（总市值和流通市值）
	if resp.PriceUsd != "" && resp.PriceUsd != "0" {
		priceDecimal, err := decimal.NewFromString(resp.PriceUsd)
		if err == nil && priceDecimal.GreaterThan(decimal.Zero) {
			// 计算总市值
			if resp.TotalSupply != "" && resp.TotalSupply != "~" && resp.TotalSupply != "0" {
				if totalSupplyDecimal, err := decimal.NewFromString(resp.TotalSupply); err == nil {
					totalMarketCap := totalSupplyDecimal.Mul(priceDecimal)
					resp.TotalMarketCap = formatMarketCap(totalMarketCap)
				}
			}

			// 计算流通市值
			if resp.CirculatingSupply != "" && resp.CirculatingSupply != "~" && resp.CirculatingSupply != "0" {
				if circulatingSupplyDecimal, err := decimal.NewFromString(resp.CirculatingSupply); err == nil {
					circulatingMarketCap := circulatingSupplyDecimal.Mul(priceDecimal)
					resp.CirculatingMarketCap = formatMarketCap(circulatingMarketCap)
				}
			}
		}
	}

	// 查询持有者数量（对于代币）
	if contractAddress != "" && l.svcCtx.ChainRpc != nil {
		l.queryTokenHolders(assetCode, network, contractAddress, chainType, resp)
	}

	l.Infof("获取代币详细信息成功: asset=%s, network=%s, contract=%s", assetCode, network, contractAddress)

	return resp, nil
}

// queryNativeTokenSupply 查询主币的总供应量和流通供应量
func (l *GetWeb3TokenDetailLogic) queryNativeTokenSupply(assetCode, network string, chainType pb.ChainRpcType, resp *pb.GetWeb3TokenDetailResp) {
	switch assetCode {
	case "TRX":
		// TRX 总供应量固定为 100,000,000,000 TRX
		resp.TotalSupply = "100000000000"
		// 尝试从链上查询流通供应量
		// 注意：直接从链上统计所有账户余额非常慢，建议使用以下方式：
		// 1. 使用第三方API（如 CoinGecko、CoinMarketCap、TronScan API）
		// 2. 定期更新缓存（如每小时更新一次）
		// 3. 从链浏览器API获取（TronScan API: https://apilist.tronscan.org/）
		circulatingSupply := l.queryTronCirculatingSupply()
		if circulatingSupply != "" {
			resp.CirculatingSupply = circulatingSupply
		} else {
			// 如果查询失败，使用已知的近似值（需要定期更新）
			resp.CirculatingSupply = "94684453212.681"
		}
	case "ETH":
		// ETH 没有固定总供应量（持续增发，但EIP-1559后部分销毁）
		resp.TotalSupply = "~"
		// 流通供应量需要从链上查询或使用第三方API
		// 推荐使用：Etherscan API、CoinGecko API
		circulatingSupply := l.queryETHCirculatingSupply()
		if circulatingSupply != "" {
			resp.CirculatingSupply = circulatingSupply
		} else {
			resp.CirculatingSupply = "~"
		}
	case "BNB":
		// BNB 总供应量固定为 200,000,000 BNB（但会定期销毁，实际流通量会减少）
		resp.TotalSupply = "200000000"
		// 流通供应量需要从链上查询或使用第三方API
		// 推荐使用：BSCScan API、CoinGecko API
		circulatingSupply := l.queryBNBCirculatingSupply()
		if circulatingSupply != "" {
			resp.CirculatingSupply = circulatingSupply
		} else {
			resp.CirculatingSupply = "~"
		}
	default:
		resp.TotalSupply = "~"
		resp.CirculatingSupply = "~"
	}
}

// queryTronCirculatingSupply 查询TRON流通供应量
// 注意：直接从链上查询很慢，建议使用第三方API或定期更新的缓存
func (l *GetWeb3TokenDetailLogic) queryTronCirculatingSupply() string {
	// 方式1: 从Redis缓存获取（如果之前有更新过）
	if l.svcCtx.RedisClient != nil {
		cacheKey := "token:supply:TRX:circulating"
		if cached, err := l.svcCtx.RedisClient.Get(l.ctx, cacheKey).Result(); err == nil && cached != "" {
			return cached
		}
	}

	// 方式2: 使用TronScan API
	url := "https://apilist.tronscan.org/api/token/overview?token=TRX"
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(l.ctx, "GET", url, nil)
	if err != nil {
		l.Debugf("Failed to create request for TRX circulating supply: %v", err)
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		l.Debugf("Failed to query TRX circulating supply from TronScan: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Debugf("TronScan API returned non-200 status: %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Debugf("Failed to read TronScan response: %v", err)
		return ""
	}

	var result struct {
		CirculatingSupply string `json:"circulatingSupply"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		l.Debugf("Failed to parse TronScan response: %v", err)
		return ""
	}

	if result.CirculatingSupply != "" {
		// 缓存结果（缓存1小时）
		if l.svcCtx.RedisClient != nil {
			cacheKey := "token:supply:TRX:circulating"
			l.svcCtx.RedisClient.Set(l.ctx, cacheKey, result.CirculatingSupply, 1*time.Hour)
		}
		return result.CirculatingSupply
	}

	return "" // 返回空字符串表示查询失败，使用默认值
}

// queryETHCirculatingSupply 查询ETH流通供应量
func (l *GetWeb3TokenDetailLogic) queryETHCirculatingSupply() string {
	// 方式1: 从Redis缓存获取
	if l.svcCtx.RedisClient != nil {
		cacheKey := "token:supply:ETH:circulating"
		if cached, err := l.svcCtx.RedisClient.Get(l.ctx, cacheKey).Result(); err == nil && cached != "" {
			return cached
		}
	}

	// 方式2: 使用CoinGecko API（免费，有速率限制）
	url := "https://api.coingecko.com/api/v3/coins/ethereum?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false"
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(l.ctx, "GET", url, nil)
	if err != nil {
		l.Debugf("Failed to create request for ETH circulating supply: %v", err)
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		l.Debugf("Failed to query ETH circulating supply from CoinGecko: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Debugf("CoinGecko API returned non-200 status: %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Debugf("Failed to read CoinGecko response: %v", err)
		return ""
	}

	var result struct {
		MarketData struct {
			CirculatingSupply float64 `json:"circulating_supply"`
		} `json:"market_data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		l.Debugf("Failed to parse CoinGecko response: %v", err)
		return ""
	}

	if result.MarketData.CirculatingSupply > 0 {
		circulatingSupply := fmt.Sprintf("%.0f", result.MarketData.CirculatingSupply)
		// 缓存结果（缓存1小时）
		if l.svcCtx.RedisClient != nil {
			cacheKey := "token:supply:ETH:circulating"
			l.svcCtx.RedisClient.Set(l.ctx, cacheKey, circulatingSupply, 1*time.Hour)
		}
		return circulatingSupply
	}

	return "" // 返回空字符串表示查询失败
}

// queryBNBCirculatingSupply 查询BNB流通供应量
func (l *GetWeb3TokenDetailLogic) queryBNBCirculatingSupply() string {
	// 方式1: 从Redis缓存获取
	if l.svcCtx.RedisClient != nil {
		cacheKey := "token:supply:BNB:circulating"
		if cached, err := l.svcCtx.RedisClient.Get(l.ctx, cacheKey).Result(); err == nil && cached != "" {
			return cached
		}
	}

	// 方式2: 使用CoinGecko API
	url := "https://api.coingecko.com/api/v3/coins/binancecoin?localization=false&tickers=false&market_data=true&community_data=false&developer_data=false"
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(l.ctx, "GET", url, nil)
	if err != nil {
		l.Debugf("Failed to create request for BNB circulating supply: %v", err)
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		l.Debugf("Failed to query BNB circulating supply from CoinGecko: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Debugf("CoinGecko API returned non-200 status: %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Debugf("Failed to read CoinGecko response: %v", err)
		return ""
	}

	var result struct {
		MarketData struct {
			CirculatingSupply float64 `json:"circulating_supply"`
		} `json:"market_data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		l.Debugf("Failed to parse CoinGecko response: %v", err)
		return ""
	}

	if result.MarketData.CirculatingSupply > 0 {
		circulatingSupply := fmt.Sprintf("%.0f", result.MarketData.CirculatingSupply)
		// 缓存结果（缓存1小时）
		if l.svcCtx.RedisClient != nil {
			cacheKey := "token:supply:BNB:circulating"
			l.svcCtx.RedisClient.Set(l.ctx, cacheKey, circulatingSupply, 1*time.Hour)
		}
		return circulatingSupply
	}

	return "" // 返回空字符串表示查询失败
}

// queryTokenCirculatingSupply 查询代币流通供应量（从第三方API）
func (l *GetWeb3TokenDetailLogic) queryTokenCirculatingSupply(network, contractAddress, assetCode string) string {
	network = strings.ToUpper(strings.TrimSpace(network))
	contractAddress = strings.TrimSpace(contractAddress)
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))

	if contractAddress == "" {
		return ""
	}

	// 先尝试从Redis缓存获取
	if l.svcCtx.RedisClient != nil {
		cacheKey := fmt.Sprintf("token:supply:%s:%s:circulating", network, contractAddress)
		if cached, err := l.svcCtx.RedisClient.Get(context.Background(), cacheKey).Result(); err == nil && cached != "" {
			return cached
		}
	}

	// 尝试使用CoinGecko API查询（需要找到对应的coin_id）
	// 注意：CoinGecko需要先通过合约地址找到对应的coin_id，这可能需要额外的API调用
	// 这里先返回空，后续可以扩展实现

	return ""
}

// queryTokenHolders 查询代币持有者数量
// 注意：直接从链上查询所有持有者非常慢，建议使用第三方API或定期更新的缓存
func (l *GetWeb3TokenDetailLogic) queryTokenHolders(assetCode, network, contractAddress string, chainType pb.ChainRpcType, resp *pb.GetWeb3TokenDetailResp) {
	// 持有者数量查询通常需要：
	// 1. 扫描所有交易日志（Transfer事件）来统计唯一地址（很慢）
	// 2. 或使用第三方API（如 Etherscan、TronScan、BSCScan）
	// 3. 或维护一个定期更新的缓存

	// 先尝试从Redis缓存获取
	if l.svcCtx.RedisClient != nil {
		cacheKey := fmt.Sprintf("token:holders:%s:%s", network, contractAddress)
		if cached, err := l.svcCtx.RedisClient.Get(l.ctx, cacheKey).Result(); err == nil && cached != "" {
			resp.Holders = cached
			return
		}
	}

	// 尝试从第三方API查询
	holders := l.queryTokenHoldersFromAPI(network, contractAddress)
	if holders != "" {
		resp.Holders = holders
		// 缓存结果（缓存1小时）
		if l.svcCtx.RedisClient != nil {
			cacheKey := fmt.Sprintf("token:holders:%s:%s", network, contractAddress)
			l.svcCtx.RedisClient.Set(l.ctx, cacheKey, holders, 1*time.Hour)
		}
		return
	}

	// 如果都查询不到，返回 "~" 表示未知
	resp.Holders = "~"
}

// queryTokenHoldersFromAPI 从第三方API查询代币持有者数量
func (l *GetWeb3TokenDetailLogic) queryTokenHoldersFromAPI(network, contractAddress string) string {
	network = strings.ToUpper(strings.TrimSpace(network))
	contractAddress = strings.TrimSpace(contractAddress)

	if contractAddress == "" {
		return ""
	}

	var url string
	var parseFunc func([]byte) (string, error)

	switch network {
	case "TRON", "TRC20":
		// TronScan API: 查询TRC20代币持有者数量
		url = fmt.Sprintf("https://apilist.tronscan.org/api/token/trc20/holders?contract=%s&limit=1", contractAddress)
		parseFunc = func(body []byte) (string, error) {
			var result struct {
				Total int64 `json:"total"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				return "", err
			}
			if result.Total > 0 {
				return formatNumberWithCommas(result.Total), nil
			}
			return "", fmt.Errorf("no holders found")
		}
	case "ETH", "ETHEREUM", "ERC20":
		// Etherscan API: 需要API Key，这里先使用免费方式
		// 注意：Etherscan免费API有速率限制，建议配置API Key
		// url = fmt.Sprintf("https://api.etherscan.io/api?module=token&action=tokenholderlist&contractaddress=%s&apikey=YOUR_API_KEY", contractAddress)
		// 暂时返回空，需要配置API Key后才能使用
		return ""
	case "BSC", "BEP20":
		// BSCScan API: 需要API Key，这里先使用免费方式
		// 注意：BSCScan免费API有速率限制，建议配置API Key
		// url = fmt.Sprintf("https://api.bscscan.com/api?module=token&action=tokenholderlist&contractaddress=%s&apikey=YOUR_API_KEY", contractAddress)
		// 暂时返回空，需要配置API Key后才能使用
		return ""
	default:
		return ""
	}

	if url == "" {
		return ""
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(l.ctx, "GET", url, nil)
	if err != nil {
		l.Debugf("Failed to create request for token holders: %v", err)
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		l.Debugf("Failed to query token holders from API: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		l.Debugf("API returned non-200 status: %d", resp.StatusCode)
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Debugf("Failed to read API response: %v", err)
		return ""
	}

	holders, err := parseFunc(body)
	if err != nil {
		l.Debugf("Failed to parse API response: %v", err)
		return ""
	}

	return holders
}

// networkToChainRpcType 将网络名称转换为 ChainRpcType
func (l *GetWeb3TokenDetailLogic) networkToChainRpcType(network string) (pb.ChainRpcType, error) {
	network = strings.ToUpper(strings.TrimSpace(network))
	switch network {
	case "TRON", "TRC20":
		return pb.ChainRpcType_CHAIN_TYPE_TRON, nil
	case "ETH", "ETHEREUM", "ERC20":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, nil
	case "BSC", "BEP20":
		return pb.ChainRpcType_CHAIN_TYPE_BSC, nil
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED, fmt.Errorf("unsupported network: %s", network)
	}
}

// isNativeToken 判断是否为主币
func (l *GetWeb3TokenDetailLogic) isNativeToken(assetCode, network string) bool {
	network = strings.ToUpper(strings.TrimSpace(network))
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))

	switch network {
	case "TRON", "TRC20":
		return assetCode == "TRX"
	case "ETH", "ETHEREUM", "ERC20":
		return assetCode == "ETH"
	case "BSC", "BEP20":
		return assetCode == "BNB"
	default:
		return false
	}
}

// getNetworkDisplayName 获取网络显示名称
func (l *GetWeb3TokenDetailLogic) getNetworkDisplayName(network string) string {
	network = strings.ToUpper(strings.TrimSpace(network))
	switch network {
	case "TRON", "TRC20":
		return "波场"
	case "ETH", "ETHEREUM", "ERC20":
		return "Ethereum"
	case "BSC", "BEP20":
		return "BSC"
	default:
		return network
	}
}

// getChainId 获取链ID
func (l *GetWeb3TokenDetailLogic) getChainId(network string) int64 {
	network = strings.ToUpper(strings.TrimSpace(network))
	switch network {
	case "TRON", "TRC20":
		return 728126428 // TRON mainnet
	case "ETH", "ETHEREUM", "ERC20":
		return 1 // Ethereum mainnet
	case "BSC", "BEP20":
		return 56 // BSC mainnet
	default:
		return 0
	}
}

// networkToChainCode 将网络名称转换为 chainCode（用于数据库查询）
func (l *GetWeb3TokenDetailLogic) networkToChainCode(network string) string {
	network = strings.ToUpper(strings.TrimSpace(network))
	switch network {
	case "TRON", "TRC20":
		return "TRON"
	case "ETH", "ETHEREUM", "ERC20":
		return "ETH"
	case "BSC", "BEP20":
		return "BSC"
	default:
		return ""
	}
}

// formatMarketCap 格式化市值（保留整数，添加千分位）
func formatMarketCap(value decimal.Decimal) string {
	// 转换为整数（美元）
	intValue := value.IntPart()
	if intValue == 0 {
		return "0"
	}

	// 添加千分位
	return formatNumberWithCommas(intValue)
}

// formatNumberWithCommas 添加千分位
func formatNumberWithCommas(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	str := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}
