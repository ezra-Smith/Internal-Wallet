package okx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"internalwallet/services/swap/rpc/internal/config"
)

type apiError struct {
	StatusCode int
	Code       string
	Msg        string
	Body       string
}

func (e *apiError) Error() string {
	if e == nil {
		return "okx api error"
	}
	if strings.TrimSpace(e.Code) != "" {
		return fmt.Sprintf("okx api error: status=%d code=%s msg=%s body=%s", e.StatusCode, e.Code, e.Msg, e.Body)
	}
	return fmt.Sprintf("okx api error: status=%d body=%s", e.StatusCode, e.Body)
}

type baseResponse struct {
	Code string          `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

type stringOrNumber struct {
	s string
}

func (v *stringOrNumber) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		v.s = ""
		return nil
	}
	if strings.HasPrefix(s, "\"") {
		var out string
		if err := json.Unmarshal(b, &out); err != nil {
			return err
		}
		v.s = out
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	v.s = n.String()
	return nil
}

func (v stringOrNumber) String() string { return v.s }

type tokenDTO struct {
	// OKX APIs use both `decimal` (quote/swap) and `decimals` (all-tokens).
	Decimal              stringOrNumber `json:"decimal"`
	Decimals             stringOrNumber `json:"decimals"`
	TokenContractAddress string         `json:"tokenContractAddress"`
	TokenLogoUrl         string         `json:"tokenLogoUrl"`
	TokenName            string         `json:"tokenName"`
	TokenSymbol          string         `json:"tokenSymbol"`
}

// dexProtocolDTO represents DEX protocol in router list
type dexProtocolDTO struct {
	DexName string `json:"dexName"`
	Percent string `json:"percent"`
}

// dexRouterTokenDTO represents token in dexRouterList
type dexRouterTokenDTO struct {
	Decimal              stringOrNumber `json:"decimal"`
	IsHoneyPot           bool           `json:"isHoneyPot"`
	TaxRate              string         `json:"taxRate"`
	TokenContractAddress string         `json:"tokenContractAddress"`
	TokenSymbol          string         `json:"tokenSymbol"`
	TokenUnitPrice       string         `json:"tokenUnitPrice"`
	// Index fields (may be present in some responses)
	//FromTokenIndex string `json:"fromTokenIndex,omitempty"`
	//ToTokenIndex   string `json:"toTokenIndex,omitempty"`
}

type dexRouterDTO struct {
	DexProtocol dexProtocolDTO    `json:"dexProtocol"`
	FromToken   dexRouterTokenDTO `json:"fromToken"`
	ToToken     dexRouterTokenDTO `json:"toToken"`
	// Optional fields that may appear in some responses
	//DexName     string            `json:"dexName,omitempty"`    // 询价路径 DEX 名称
	//DexLogo     string            `json:"dexLogo,omitempty"`    // DEX 协议 logo
	//TradeFee    string            `json:"tradeFee,omitempty"`   // 询价路径预估消耗的网络费用
	//AmountOut   string            `json:"amountOut,omitempty"`  // 询价路径的接收数量
	FromTokenIndex string `json:"fromTokenIndex,omitempty"`
	ToTokenIndex   string `json:"toTokenIndex,omitempty"`
}

// quoteTokenDTO represents token information in quote response
type quoteTokenDTO struct {
	Decimal              stringOrNumber `json:"decimal"`
	IsHoneyPot           bool           `json:"isHoneyPot"`
	TaxRate              string         `json:"taxRate"`
	TokenContractAddress string         `json:"tokenContractAddress"`
	TokenSymbol          string         `json:"tokenSymbol"`
	TokenUnitPrice       string         `json:"tokenUnitPrice"`
}

type quoteDataDTO struct {
	ChainIndex         string         `json:"chainIndex"`
	ContextSlot        stringOrNumber `json:"contextSlot,omitempty"` // Solana specific
	FromTokenAmount    string         `json:"fromTokenAmount"`
	ToTokenAmount      string         `json:"toTokenAmount"`
	EstimateGasFee     stringOrNumber `json:"estimateGasFee"`
	Router             string         `json:"router"`
	SwapMode           string         `json:"swapMode"`
	TradeFee           string         `json:"tradeFee"`
	PriceImpactPercent string         `json:"priceImpactPercent"`
	DexName            string         `json:"dexName,omitempty"` // 询价路径 DEX 名称
	DexLogo            string         `json:"dexLogo,omitempty"` // DEX 协议 logo
	FromToken          quoteTokenDTO  `json:"fromToken"`
	ToToken            quoteTokenDTO  `json:"toToken"`
	DexRouterList      []dexRouterDTO `json:"dexRouterList"`
}

type routerResultDTO struct {
	ChainIndex      string         `json:"chainIndex"`
	FromToken       tokenDTO       `json:"fromToken"`
	ToToken         tokenDTO       `json:"toToken"`
	FromTokenAmount string         `json:"fromTokenAmount"`
	ToTokenAmount   string         `json:"toTokenAmount"`
	EstimateGas     stringOrNumber `json:"estimateGas"`
	EstimateGasFee  stringOrNumber `json:"estimateGasFee"`
	DexRouterList   []dexRouterDTO `json:"dexRouterList"`
	TradeFee        string         `json:"tradeFee"` // 询价路径预估消耗的网络费用 (USD 计价)
	SwapMode        string         `json:"swapMode"`
}

// 发交易信息
// txDTO represents transaction data in swap response
type txDTO struct {
	Data  string         `json:"data"`  //Call data
	From  string         `json:"from"`  //用户钱包地址
	To    string         `json:"to"`    //欧易 DEX router 合约地址
	Value stringOrNumber `json:"value"` //与合约交互的主链币数量，以 10 进制标准格式最小单位返回 (wei)
	Gas   stringOrNumber `json:"gas"`   //gas 费限值的估计值，在 gasprice 基础上增加 50%，以 10 进制标准格式返回
	//GasLimit             stringOrNumber `json:"gasLimit"`
	GasPrice             stringOrNumber `json:"gasPrice"`                   //以 wei 为单位的 gas price ，以 10 进制标准格式返回
	MaxPriorityFeePerGas stringOrNumber `json:"maxPriorityFeePerGas"`       //EIP-1559:每单位 gas 优先费用的推荐值
	MaxSpendAmount       stringOrNumber `json:"maxSpendAmount,omitempty"`   //达到滑点上限时可花费的询价代币最大数量（适用于 exactOut 模式）
	MinReceiveAmount     stringOrNumber `json:"minReceiveAmount,omitempty"` //目标币种的最小兑换数量 (兑换价格达到滑点限制的极限值时，目标币种的兑换数量，如：900645839798)
	SlippagePercent      string         `json:"slippagePercent"`            //当前交易的滑点值
	SignatureData        []string       `json:"signatureData,omitempty"`    //额外的签名数据（如果返回此参数，则代表该交易需要额外的签名数据）
}

type swapDataDTO struct {
	//ChainIndex         string          `json:"chainIndex"`
	//DexContractAddress string          `json:"dexContractAddress"`
	RouterResult routerResultDTO `json:"routerResult"`
	Tx           txDTO           `json:"tx"`
}

type approveTxDataDTO struct {
	ChainIndex         string         `json:"chainIndex"`
	Data               string         `json:"data"`
	DexContractAddress string         `json:"dexContractAddress"`
	GasLimit           stringOrNumber `json:"gasLimit"`
	GasPrice           stringOrNumber `json:"gasPrice"`
}

// ============================================================================
// New DTO types for OKX DEX API endpoints
// ============================================================================

// supportedChainDTO represents a supported chain from OKX
type supportedChainDTO struct {
	ChainIndex             int64  `json:"chainIndex"`
	ChainName              string `json:"chainName"`
	DexTokenApproveAddress string `json:"dexTokenApproveAddress"`
}

// liquiditySourceDTO represents a DEX/liquidity source from OKX
type liquiditySourceDTO struct {
	ID   string `json:"id"`
	Logo string `json:"logo"`
	Name string `json:"name"`
}

// swapHistoryDTO represents swap history from OKX
type swapHistoryDTO struct {
	ChainIndex       string         `json:"chainIndex"`
	TxHash           string         `json:"txHash"`
	Height           string         `json:"height"`
	TxTime           stringOrNumber `json:"txTime"`
	Status           string         `json:"status"` // "pending", "success", "fail"
	TxType           string         `json:"txType"` // Approve, Wrap, Unwrap, Swap
	FromAddress      string         `json:"fromAddress"`
	DexRouter        string         `json:"dexRouter"` // 根据官方文档响应示例，字段名是 dexRouter
	ToAddress        string         `json:"toAddress"`
	FromTokenDetails tokenDetailDTO `json:"fromTokenDetails"` // 单个对象，不是数组
	ToTokenDetails   tokenDetailDTO `json:"toTokenDetails"`   // 单个对象，不是数组
	ReferralAmount   string         `json:"referralAmount"`   // 注意：响应示例中是 referralAmount (两个r)
	ErrorMsg         string         `json:"errorMsg"`
	GasLimit         stringOrNumber `json:"gasLimit"`
	GasUsed          stringOrNumber `json:"gasUsed"`
	GasPrice         stringOrNumber `json:"gasPrice"`
	TxFee            string         `json:"txFee"`
}

// tokenDetailDTO represents token details in swap history
type tokenDetailDTO struct {
	Symbol       string `json:"symbol"`
	Amount       string `json:"amount"`
	TokenAddress string `json:"tokenAddress"`
}

type Client struct {
	baseURL  string
	basePath string

	apiKey     string
	secretKey  string
	passphrase string

	httpClient *http.Client
	limiter    *rate.Limiter

	maxRetries     int
	retryBaseDelay time.Duration
}

func NewClient(cfg config.OkxConfig) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://web3.okx.com"
	}

	basePath := strings.TrimSpace(cfg.BasePath)
	if basePath == "" {
		basePath = "/api/v6/dex/aggregator"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimRight(basePath, "/")

	timeout := time.Duration(cfg.TimeoutMillis) * time.Millisecond
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	var limiter *rate.Limiter
	if cfg.RateLimitRPS > 0 {
		limiter = rate.NewLimiter(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitRPS)
	}

	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	retryBaseDelay := time.Duration(cfg.RetryBaseDelayMillis) * time.Millisecond
	if retryBaseDelay <= 0 {
		retryBaseDelay = 200 * time.Millisecond
	}

	return &Client{
		baseURL:        baseURL,
		basePath:       basePath,
		apiKey:         strings.TrimSpace(cfg.ApiKey),
		secretKey:      strings.TrimSpace(cfg.SecretKey),
		passphrase:     strings.TrimSpace(cfg.Passphrase),
		httpClient:     &http.Client{Timeout: timeout},
		limiter:        limiter,
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
	}
}

// QuoteRequestOptions holds optional parameters for GetQuote
type QuoteRequestOptions struct {
	DexIds                       string
	DirectRoute                  bool
	PriceImpactProtectionPercent string
	FeePercent                   string
}

func (c *Client) GetQuote(ctx context.Context, chainID int64, src, dst, amount string, options *QuoteRequestOptions) (*quoteDataDTO, error) {
	q := url.Values{}
	q.Set("chainIndex", fmt.Sprintf("%d", chainID))
	q.Set("amount", strings.TrimSpace(amount))
	q.Set("fromTokenAddress", strings.TrimSpace(src))
	q.Set("toTokenAddress", strings.TrimSpace(dst))
	q.Set("swapMode", "exactIn")

	if options != nil {
		if options.DexIds != "" {
			q.Set("dexIds", options.DexIds)
		}
		if options.DirectRoute {
			q.Set("directRoute", "true")
		}
		if options.PriceImpactProtectionPercent != "" {
			q.Set("priceImpactProtectionPercent", options.PriceImpactProtectionPercent)
		}
		if options.FeePercent != "" {
			q.Set("feePercent", options.FeePercent)
		}
	}

	path := c.basePath + "/quote"
	var items []quoteDataDTO
	if err := c.getDataJSON(ctx, path, q, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("empty quote response")
	}
	return &items[0], nil
}

// SwapRequestOptions holds optional parameters for GetSwap
type SwapRequestOptions struct {
	FeePercent                   string
	GasLimit                     string
	GasLevel                     string
	DexIds                       string
	ExcludeDexIds                string
	PriceImpactProtectionPercent string
}

func (c *Client) GetSwap(ctx context.Context, chainID int64, src, dst, amount, userWalletAddress, swapReceiverAddress string, slippageBps int32, options *SwapRequestOptions) (*swapDataDTO, error) {
	q := url.Values{}
	q.Set("chainIndex", fmt.Sprintf("%d", chainID))
	q.Set("amount", strings.TrimSpace(amount))
	q.Set("fromTokenAddress", strings.TrimSpace(src))
	q.Set("toTokenAddress", strings.TrimSpace(dst))
	q.Set("userWalletAddress", strings.TrimSpace(userWalletAddress))
	q.Set("swapMode", "exactIn")
	q.Set("slippagePercent", formatBpsToPercent(slippageBps))
	if receiver := strings.TrimSpace(swapReceiverAddress); receiver != "" {
		q.Set("swapReceiverAddress", receiver)
	}

	if options != nil {
		if options.FeePercent != "" {
			q.Set("feePercent", options.FeePercent)
		}
		if options.GasLimit != "" {
			q.Set("gasLimit", options.GasLimit)
		}
		if options.GasLevel != "" {
			q.Set("gasLevel", options.GasLevel)
		}
		if options.DexIds != "" {
			q.Set("dexIds", options.DexIds)
		}
		if options.ExcludeDexIds != "" {
			q.Set("excludeDexIds", options.ExcludeDexIds)
		}
		if options.PriceImpactProtectionPercent != "" {
			q.Set("priceImpactProtectionPercent", options.PriceImpactProtectionPercent)
		}
	}

	path := c.basePath + "/swap"
	var items []swapDataDTO
	if err := c.getDataJSON(ctx, path, q, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("empty swap response")
	}
	return &items[0], nil
}

// GetTokens retrieves all tokens for a given chain
// API: GET /api/v6/dex/aggregator/all-tokens?chainIndex={chainIndex}
func (c *Client) GetTokens(ctx context.Context, chainID int64) ([]tokenDTO, error) {
	q := url.Values{}
	q.Set("chainIndex", fmt.Sprintf("%d", chainID))

	path := c.basePath + "/all-tokens"
	var items []tokenDTO
	if err := c.getDataJSON(context.Background(), path, q, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []tokenDTO{}, nil
	}
	return items, nil
}

// GetAllTokens is deprecated; use GetTokens instead
func (c *Client) GetAllTokens(ctx context.Context, chainID int64) ([]tokenDTO, error) {
	return c.GetTokens(ctx, chainID)
}

func (c *Client) GetApproveTransaction(ctx context.Context, chainID int64, tokenAddress, approveAmount string) (*approveTxDataDTO, error) {
	q := url.Values{}
	q.Set("chainIndex", fmt.Sprintf("%d", chainID))
	q.Set("tokenContractAddress", strings.TrimSpace(tokenAddress))
	if strings.TrimSpace(approveAmount) != "" {
		q.Set("approveAmount", strings.TrimSpace(approveAmount))
	}

	path := c.basePath + "/approve-transaction"
	var items []approveTxDataDTO
	if err := c.getDataJSON(ctx, path, q, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("empty approve response")
	}
	return &items[0], nil
}

func (c *Client) getDataJSON(ctx context.Context, path string, query url.Values, out any) error {
	body, statusCode, err := c.doRequest(context.Background(), http.MethodGet, path, query)
	if err != nil {
		return err
	}

	var resp baseResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if strings.TrimSpace(resp.Code) != "" && resp.Code != "0" {
		return &apiError{
			StatusCode: statusCode,
			Code:       strings.TrimSpace(resp.Code),
			Msg:        strings.TrimSpace(resp.Msg),
			Body:       string(body),
		}
	}
	if out == nil {
		return nil
	}
	if len(resp.Data) == 0 || strings.TrimSpace(string(resp.Data)) == "null" {
		return nil
	}
	return json.Unmarshal(resp.Data, out)
}

func (c *Client) doRequest(ctx context.Context, method string, path string, query url.Values) ([]byte, int, error) {
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, 0, err
		}
	}
	if strings.TrimSpace(c.apiKey) == "" || strings.TrimSpace(c.secretKey) == "" || strings.TrimSpace(c.passphrase) == "" {
		return nil, 0, fmt.Errorf("okx credentials not configured")
	}

	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, 0, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	pathWithQuery := u.Path

	if u.RawQuery != "" {
		pathWithQuery = pathWithQuery + "?" + u.RawQuery
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		sign := signOKX(ts, method, pathWithQuery, "", c.secretKey)

		req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("OK-ACCESS-KEY", c.apiKey)
		req.Header.Set("OK-ACCESS-SIGN", sign)
		req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
		req.Header.Set("OK-ACCESS-PASSPHRASE", c.passphrase)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.maxRetries {
				time.Sleep(c.retryBaseDelay * time.Duration(1<<attempt))
				continue
			}
			return nil, 0, err
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, resp.StatusCode, nil
		}

		apiErr := &apiError{StatusCode: resp.StatusCode, Body: string(body)}
		lastErr = apiErr
		if attempt < c.maxRetries && isRetryableStatus(resp.StatusCode) {
			time.Sleep(c.retryBaseDelay * time.Duration(1<<attempt))
			continue
		}
		return nil, resp.StatusCode, apiErr
	}

	return nil, 0, lastErr
}

func signOKX(timestamp string, method string, requestPathWithQuery string, body string, secretKey string) string {
	prehash := timestamp + strings.ToUpper(method) + requestPathWithQuery + body
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(prehash))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func isRetryableStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500 && code <= 599
}

func formatBpsToPercent(bps int32) string {
	if bps <= 0 {
		return ""
	}
	// bps: 1 = 0.01%
	val := float64(bps) / 100.0
	s := strconv.FormatFloat(val, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" {
		s = "0"
	}
	return s
}

// ============================================================================
// New OKX DEX API Methods
// ============================================================================

// GetSupportedChains retrieves the list of chains supported by OKX DEX
// API: GET /api/v6/dex/aggregator/supported/chain
func (c *Client) GetSupportedChains(ctx context.Context) ([]supportedChainDTO, error) {
	path := c.basePath + "/supported/chain"
	var items []supportedChainDTO
	if err := c.getDataJSON(ctx, path, nil, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []supportedChainDTO{}, nil
	}
	return items, nil
}

// GetLiquiditySources retrieves the list of DEX/liquidity sources for a given chain
// API: GET /api/v6/dex/aggregator/get-liquidity?chainIndex={chainIndex}
func (c *Client) GetLiquiditySources(ctx context.Context, chainID int64) ([]liquiditySourceDTO, error) {
	q := url.Values{}
	q.Set("chainIndex", fmt.Sprintf("%d", chainID))

	path := c.basePath + "/get-liquidity"
	var items []liquiditySourceDTO
	if err := c.getDataJSON(ctx, path, q, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []liquiditySourceDTO{}, nil
	}
	return items, nil
}

// GetSwapHistory retrieves the status of a swap transaction by tx hash
// API: GET /api/v6/dex/aggregator/history?chainIndex={chainIndex}&txHash={txHash}
func (c *Client) GetSwapHistory(ctx context.Context, chainID int64, txHash string, isFromMyProject bool) (*swapHistoryDTO, error) {
	q := url.Values{}
	q.Set("chainIndex", fmt.Sprintf("%d", chainID))
	q.Set("txHash", strings.TrimSpace(txHash))
	if isFromMyProject {
		q.Set("isFromMyProject", "true")
	}

	path := c.basePath + "/history"
	// 注意：根据官方文档响应示例，data字段是单个对象，不是数组
	var item swapHistoryDTO
	if err := c.getDataJSON(ctx, path, q, &item); err != nil {
		return nil, err
	}
	return &item, nil
}
