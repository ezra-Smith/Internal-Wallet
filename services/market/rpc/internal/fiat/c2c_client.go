package fiat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultC2CProvider = "binance_c2c"
)

type C2CClient struct {
	httpClient *http.Client
	baseURL    string
	endpoint   string
	retries    int
}

func NewC2CClient(baseURL, endpoint string, timeout time.Duration, retries int) *C2CClient {
	if retries < 0 {
		retries = 0
	}
	return &C2CClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL:  strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		endpoint: strings.TrimSpace(endpoint),
		retries:  retries,
	}
}

type c2cRequest struct {
	Assets       []string `json:"assets"`
	FiatCurrency string   `json:"fiatCurrency"`
	TradeType    string   `json:"tradeType"` // "BUY" or "SELL"
	FromUserRole string   `json:"fromUserRole"`
}

type c2cResponse struct {
	Code    string      `json:"code"`
	Message interface{} `json:"message"`
	Data    []c2cPrice  `json:"data"`
	Success bool        `json:"success"`
}

type c2cPrice struct {
	Asset          string  `json:"asset"`
	Currency       string  `json:"currency"`
	CurrencySymbol string  `json:"currencySymbol"`
	ReferencePrice float64 `json:"referencePrice"`
}

type FiatQuote struct {
	Asset          string
	Currency       string
	CurrencySymbol string
	BuyPrice       decimal.Decimal
	SellPrice      decimal.Decimal
	MidPrice       decimal.Decimal
	Provider       string
	TimestampMs    int64
}

func (c *C2CClient) GetBothPrices(ctx context.Context, asset, fiatCurrency string) (*FiatQuote, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	fiatCurrency = strings.ToUpper(strings.TrimSpace(fiatCurrency))
	if asset == "" || fiatCurrency == "" {
		return nil, fmt.Errorf("asset and fiatCurrency are required")
	}

	buy, buyMeta, err := c.getPrice(ctx, asset, fiatCurrency, "BUY")
	if err != nil {
		return nil, fmt.Errorf("get buy price: %w", err)
	}
	sell, sellMeta, err := c.getPrice(ctx, asset, fiatCurrency, "SELL")
	if err != nil {
		return nil, fmt.Errorf("get sell price: %w", err)
	}

	if !buy.GreaterThan(decimal.Zero) || !sell.GreaterThan(decimal.Zero) {
		return nil, fmt.Errorf("invalid C2C prices: buy=%s sell=%s", buy.String(), sell.String())
	}

	mid := buy.Add(sell).Div(decimal.NewFromInt(2))
	nowMs := time.Now().UnixMilli()

	quote := &FiatQuote{
		Asset:          asset,
		Currency:       fiatCurrency,
		CurrencySymbol: firstNonEmpty(buyMeta.currencySymbol, sellMeta.currencySymbol),
		BuyPrice:       buy,
		SellPrice:      sell,
		MidPrice:       mid,
		Provider:       defaultC2CProvider,
		TimestampMs:    nowMs,
	}

	return quote, nil
}

type c2cMeta struct {
	currencySymbol string
}

func (c *C2CClient) getPrice(ctx context.Context, asset, fiatCurrency, tradeType string) (decimal.Decimal, c2cMeta, error) {
	reqBody := c2cRequest{
		Assets:       []string{asset},
		FiatCurrency: fiatCurrency,
		TradeType:    tradeType,
		FromUserRole: "USER",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return decimal.Zero, c2cMeta{}, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + c.endpoint
	var lastErr error

	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * time.Second
			logx.Infof("Retrying C2C request (%s/%s %s), attempt=%d/%d, delay=%s", asset, fiatCurrency, tradeType, attempt+1, c.retries+1, delay)
			select {
			case <-ctx.Done():
				return decimal.Zero, c2cMeta{}, ctx.Err()
			case <-time.After(delay):
			}
		}

		price, meta, err := c.doRequest(ctx, url, bodyBytes, asset, fiatCurrency, tradeType)
		if err == nil {
			return price, meta, nil
		}
		lastErr = err
	}

	return decimal.Zero, c2cMeta{}, lastErr
}

func (c *C2CClient) doRequest(ctx context.Context, url string, bodyBytes []byte, asset, fiatCurrency, tradeType string) (decimal.Decimal, c2cMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return decimal.Zero, c2cMeta{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "internalwallet-market/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return decimal.Zero, c2cMeta{}, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return decimal.Zero, c2cMeta{}, fmt.Errorf("http status=%d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var r c2cResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return decimal.Zero, c2cMeta{}, fmt.Errorf("decode response: %w", err)
	}

	if !r.Success || r.Code != "000000" {
		return decimal.Zero, c2cMeta{}, fmt.Errorf("api error: code=%s message=%v", r.Code, r.Message)
	}
	if len(r.Data) == 0 {
		return decimal.Zero, c2cMeta{}, fmt.Errorf("no data for %s/%s %s", asset, fiatCurrency, tradeType)
	}

	p := r.Data[0]
	price := decimal.NewFromFloat(p.ReferencePrice)
	if !price.GreaterThan(decimal.Zero) {
		return decimal.Zero, c2cMeta{}, fmt.Errorf("invalid price for %s/%s %s: %v", asset, fiatCurrency, tradeType, p.ReferencePrice)
	}

	return price, c2cMeta{currencySymbol: strings.TrimSpace(p.CurrencySymbol)}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
