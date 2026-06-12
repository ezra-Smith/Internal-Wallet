package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type ItrxClient struct {
	baseURL   string
	apiKey    string
	apiSecret string
	client    *http.Client
}

func NewItrxClient(baseURL string, apiKey string, apiSecret string, timeout time.Duration) *ItrxClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ItrxClient{
		baseURL:   baseURL,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		client:    &http.Client{Timeout: timeout},
	}
}

type ItrxPlatformData struct {
	PlatformAvailEnergy int   `json:"platform_avail_energy"`
	PlatformMaxEnergy   int   `json:"platform_max_energy"`
	MinimumOrderEnergy  int   `json:"minimum_order_energy"`
	MaximumOrderEnergy  int   `json:"maximum_order_energy"`
	Balance             int64 `json:"balance"`
}

type ItrxOrderResponse struct {
	Errno   int    `json:"errno"`
	Message string `json:"message,omitempty"`
	Serial  string `json:"serial,omitempty"`
	Amount  int64  `json:"amount,omitempty"`
	Balance int64  `json:"balance,omitempty"`
}

type ItrxOrderQueryResponse struct {
	Errno   int    `json:"errno"`
	Message string `json:"message"`

	OrderNo         string  `json:"order_no"`
	ReceiveAddress  string  `json:"receive_address"`
	EnergyAmount    int     `json:"energy_amount"`
	PayAmount       float64 `json:"pay_amount"`
	Period          int     `json:"period"`
	Status          int     `json:"status"`
	RefundAmount    int     `json:"refund_amount"`
	CreateTime      string  `json:"create_time"`
	DelegateHash    string  `json:"delegate_hash,omitempty"`
	ReclaimHash     string  `json:"reclaim_hash,omitempty"`
	ReclaimTimeReal string  `json:"reclaim_time_real,omitempty"`
}

func (c *ItrxClient) GetPlatformData(ctx context.Context) (*ItrxPlatformData, []byte, error) {
	url := fmt.Sprintf("%s/api/v1/frontend/index-data", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("API-KEY", c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, body, fmt.Errorf("itrx GetPlatformData http=%d body=%s", resp.StatusCode, string(body))
	}
	var out ItrxPlatformData
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, body, err
	}
	return &out, body, nil
}

func (c *ItrxClient) CreateOrder(ctx context.Context, receiverAddress string, energyAmount int, period string) (*ItrxOrderResponse, []byte, error) {
	url := fmt.Sprintf("%s/api/v1/frontend/order", c.baseURL)
	payload := map[string]any{
		"energy_amount":   energyAmount,
		"period":          period,
		"receive_address": receiverAddress,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	signature := hmacSha256Hex(ts+"&"+string(bodyBytes), c.apiSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("API-KEY", c.apiKey)
	req.Header.Set("TIMESTAMP", ts)
	req.Header.Set("SIGNATURE", signature)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, respBytes, fmt.Errorf("itrx CreateOrder http=%d body=%s", resp.StatusCode, string(respBytes))
	}
	var out ItrxOrderResponse
	if err := json.Unmarshal(respBytes, &out); err != nil {
		return nil, respBytes, err
	}
	if out.Errno != 0 {
		return &out, respBytes, fmt.Errorf("itrx CreateOrder errno=%d msg=%s", out.Errno, out.Message)
	}
	return &out, respBytes, nil
}

func (c *ItrxClient) QueryOrder(ctx context.Context, serial string) (*ItrxOrderQueryResponse, []byte, error) {
	url := fmt.Sprintf("%s/api/v1/frontend/order/query?serial=%s", c.baseURL, serial)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("API-KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, body, fmt.Errorf("itrx QueryOrder http=%d body=%s", resp.StatusCode, string(body))
	}
	var out ItrxOrderQueryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, body, err
	}
	if out.Errno != 0 {
		return &out, body, fmt.Errorf("itrx QueryOrder errno=%d msg=%s", out.Errno, out.Message)
	}
	return &out, body, nil
}

func ValidatePlatformData(data *ItrxPlatformData, requestedEnergy int) error {
	if data == nil {
		return fmt.Errorf("platform data is nil")
	}
	if requestedEnergy <= 0 {
		return fmt.Errorf("invalid requestedEnergy: %d", requestedEnergy)
	}
	if data.PlatformAvailEnergy > 0 && data.PlatformAvailEnergy < requestedEnergy {
		return fmt.Errorf("platform avail energy insufficient: want=%d avail=%d", requestedEnergy, data.PlatformAvailEnergy)
	}
	if data.MinimumOrderEnergy > 0 && requestedEnergy < data.MinimumOrderEnergy {
		return fmt.Errorf("requested energy below minimum: want=%d min=%d", requestedEnergy, data.MinimumOrderEnergy)
	}
	if data.MaximumOrderEnergy > 0 && requestedEnergy > data.MaximumOrderEnergy {
		return fmt.Errorf("requested energy above maximum: want=%d max=%d", requestedEnergy, data.MaximumOrderEnergy)
	}
	if data.PlatformMaxEnergy > 0 && requestedEnergy > data.PlatformMaxEnergy {
		return fmt.Errorf("requested energy above platform max: want=%d max=%d", requestedEnergy, data.PlatformMaxEnergy)
	}
	return nil
}

func hmacSha256Hex(payload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
