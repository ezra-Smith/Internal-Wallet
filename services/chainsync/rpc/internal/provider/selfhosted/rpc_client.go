package selfhosted

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

func (p *SelfHostedProvider) callRPC(ctx context.Context, method string, params []interface{}) (*RPCResponse, error) {
	rpcReq := RPCRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      int(time.Now().Unix() % 10000),
	}

	reqBody, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.config.APIKey != "" {
		req.Header.Set("X-API-Key", p.config.APIKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &rpcResp, nil
}

func (p *SelfHostedProvider) callTronAPIRaw(ctx context.Context, endpoint string, payload interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s%s", p.endpoint, endpoint)

	var req *http.Request
	var err error

	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal TRON request: %w", err)
		}
		req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create TRON POST request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create TRON GET request: %w", err)
		}
	}

	if p.config.APIKey != "" {
		req.Header.Set("X-API-Key", p.config.APIKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TRON API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read TRON API response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TRON API returned status %d", resp.StatusCode)
	}

	logx.Debugf("TRON API Response: %s", string(body))
	return body, nil
}

