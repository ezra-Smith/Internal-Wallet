package selfhosted

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"internalwallet/proto/pb"
)

// getInternalTransactions retrieves internal transfers via debug_traceTransaction (if supported).
func (p *SelfHostedProvider) getInternalTransactions(ctx context.Context, txHash string) ([]*pb.InternalTransaction, error) {
	resp, err := p.callRPC(ctx, "debug_traceTransaction", []interface{}{
		txHash,
		map[string]interface{}{"tracer": "callTracer"},
	})
	if err != nil {
		if strings.Contains(err.Error(), "method not found") || strings.Contains(err.Error(), "not supported") {
			logx.Debugf("Node does not support debug_traceTransaction for %s, skipping internal transactions", txHash)
			return []*pb.InternalTransaction{}, nil
		}
		return nil, fmt.Errorf("failed to trace transaction %s: %v", txHash, err)
	}

	var traceResult map[string]interface{}
	if err := json.Unmarshal(resp.Result, &traceResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trace result: %v", err)
	}

	internalTxs := make([]*pb.InternalTransaction, 0)

	var parseCall func(call map[string]interface{}, depth uint64)
	parseCall = func(call map[string]interface{}, depth uint64) {
		internalTx := &pb.InternalTransaction{TraceAddressIndex: depth}

		if from, ok := call["from"].(string); ok {
			internalTx.FromAddress = from
		}
		if to, ok := call["to"].(string); ok {
			internalTx.ToAddress = to
		}
		if value, ok := call["value"].(string); ok {
			internalTx.Value = value
		}
		if gas, ok := call["gas"].(string); ok {
			internalTx.Gas = gas
		}
		if gasUsed, ok := call["gasUsed"].(string); ok {
			if gu, err := strconv.ParseInt(gasUsed[2:], 16, 64); err == nil {
				internalTx.GasUsed = uint64(gu)
			}
		}
		if input, ok := call["input"].(string); ok {
			internalTx.Input = input
		}
		if output, ok := call["output"].(string); ok {
			internalTx.Output = output
		}
		if callType, ok := call["type"].(string); ok {
			internalTx.Type = callType
		}
		if errorMsg, ok := call["error"].(string); ok {
			internalTx.Error = errorMsg
		}

		if internalTx.Value != "" && internalTx.Value != "0x0" && internalTx.Value != "0" {
			internalTxs = append(internalTxs, internalTx)
		}

		if calls, ok := call["calls"].([]interface{}); ok {
			for _, subCallInterface := range calls {
				if subCall, ok := subCallInterface.(map[string]interface{}); ok {
					parseCall(subCall, depth+1)
				}
			}
		}
	}

	parseCall(traceResult, 0)

	logx.Debugf("🔍 Parsed %d internal transactions for %s", len(internalTxs), txHash)
	return internalTxs, nil
}

