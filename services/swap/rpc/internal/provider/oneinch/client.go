package oneinch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"internalwallet/services/swap/rpc/internal/config"
)

type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("1inch api error: status=%d body=%s", e.StatusCode, e.Body)
}

type Client struct {
	baseURL      string
	swapBasePath string
	apiKey       string

	httpClient *http.Client
	limiter    *rate.Limiter

	maxRetries     int
	retryBaseDelay time.Duration
}

func NewClient(cfg config.OneInchConfig) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.1inch.dev"
	}
	swapBasePath := strings.TrimSpace(cfg.SwapBasePath)
	if swapBasePath == "" {
		swapBasePath = "/swap/v6.0"
	}
	if !strings.HasPrefix(swapBasePath, "/") {
		swapBasePath = "/" + swapBasePath
	}

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
		swapBasePath:   swapBasePath,
		apiKey:         strings.TrimSpace(cfg.ApiKey),
		httpClient:     &http.Client{Timeout: timeout},
		limiter:        limiter,
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
	}
}

type tokenDTO struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Decimals uint32 `json:"decimals"`
	LogoURI  string `json:"logoURI"`
}

type stringOrNumber struct {
	s string
}

func (v *stringOrNumber) UnmarshalJSON(b []byte) error {
	b = []byte(strings.TrimSpace(string(b)))
	if len(b) == 0 || string(b) == "null" {
		v.s = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		v.s = s
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	v.s = n.String()
	return nil
}

func (v stringOrNumber) String() string {
	return v.s
}

type quoteResponseDTO struct {
	FromToken    tokenDTO `json:"fromToken"`
	ToToken      tokenDTO `json:"toToken"`
	FromAmount   string   `json:"fromAmount"`
	ToAmount     string   `json:"toAmount"`
	EstimatedGas uint64   `json:"estimatedGas"`
	Protocols    any      `json:"protocols"`
}

type txDTO struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Data     string         `json:"data"`
	Value    string         `json:"value"`
	Gas      stringOrNumber `json:"gas"`
	GasPrice stringOrNumber `json:"gasPrice"`
}

type swapResponseDTO struct {
	FromToken    tokenDTO `json:"fromToken"`
	ToToken      tokenDTO `json:"toToken"`
	FromAmount   string   `json:"fromAmount"`
	ToAmount     string   `json:"toAmount"`
	EstimatedGas uint64   `json:"estimatedGas"`
	Protocols    any      `json:"protocols"`
	Tx           txDTO    `json:"tx"`
}

type tokensResponseDTO struct {
	Tokens map[string]tokenDTO `json:"tokens"`
}

type allowanceResponseDTO struct {
	Allowance string `json:"allowance"`
}

type approveTxResponseDTO struct {
	Data     string         `json:"data"`
	Gas      stringOrNumber `json:"gas"`
	GasPrice stringOrNumber `json:"gasPrice"`
	To       string         `json:"to"`
	Value    string         `json:"value"`
}

func (c *Client) GetQuote(ctx context.Context, chainID int64, src, dst, amount string, includeProtocols bool) (*quoteResponseDTO, error) {
	q := url.Values{}
	q.Set("src", src)
	q.Set("dst", dst)
	q.Set("amount", amount)
	if includeProtocols {
		q.Set("includeProtocols", "true")
	}

	path := fmt.Sprintf("%s/%d/quote", c.swapBasePath, chainID)
	var out quoteResponseDTO
	if err := c.getJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetSwap(ctx context.Context, chainID int64, src, dst, amount, from, receiver string, slippageBps int32) (*swapResponseDTO, error) {
	q := url.Values{}
	q.Set("src", src)
	q.Set("dst", dst)
	q.Set("amount", amount)
	q.Set("from", from)
	if receiver != "" {
		q.Set("receiver", receiver)
	}
	// 1inch expects slippage in percent (e.g. 1 -> 1%).
	if slippageBps > 0 {
		q.Set("slippage", fmt.Sprintf("%.2f", float64(slippageBps)/100.0))
	}

	path := fmt.Sprintf("%s/%d/swap", c.swapBasePath, chainID)
	var out swapResponseDTO
	if err := c.getJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetTokens(ctx context.Context, chainID int64) (*tokensResponseDTO, error) {
	path := fmt.Sprintf("%s/%d/tokens", c.swapBasePath, chainID)
	var out tokensResponseDTO
	if err := c.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetAllowance(ctx context.Context, chainID int64, tokenAddress, walletAddress string) (*allowanceResponseDTO, error) {
	q := url.Values{}
	q.Set("tokenAddress", tokenAddress)
	q.Set("walletAddress", walletAddress)
	path := fmt.Sprintf("%s/%d/approve/allowance", c.swapBasePath, chainID)
	var out allowanceResponseDTO
	if err := c.getJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetApproveTransaction(ctx context.Context, chainID int64, tokenAddress, amount string) (*approveTxResponseDTO, error) {
	q := url.Values{}
	q.Set("tokenAddress", tokenAddress)
	if strings.TrimSpace(amount) != "" {
		q.Set("amount", strings.TrimSpace(amount))
	}
	path := fmt.Sprintf("%s/%d/approve/transaction", c.swapBasePath, chainID)
	var out approveTxResponseDTO
	if err := c.getJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	if out == nil {
		return fmt.Errorf("nil output")
	}
	b, err := c.doRequest(ctx, http.MethodGet, path, query)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (c *Client) doRequest(ctx context.Context, method string, path string, query url.Values) ([]byte, error) {
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.maxRetries {
				time.Sleep(c.retryBaseDelay * time.Duration(1<<attempt))
				continue
			}
			return nil, err
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}

		apiErr := &apiError{StatusCode: resp.StatusCode, Body: string(body)}
		lastErr = apiErr
		if attempt < c.maxRetries && isRetryableStatus(resp.StatusCode) {
			time.Sleep(c.retryBaseDelay * time.Duration(1<<attempt))
			continue
		}
		return nil, apiErr
	}

	return nil, lastErr
}

func isRetryableStatus(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500 && code <= 599
}
