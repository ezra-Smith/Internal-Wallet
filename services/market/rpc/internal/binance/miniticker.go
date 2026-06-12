package binance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var errEmptyMessage = errors.New("empty message")

type miniTickerEvent struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Close     string `json:"c"`
}

type combinedStreamEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

func parseMiniTickerPayload(msg []byte) ([]miniTickerEvent, int64, error) {
	msg = bytes.TrimSpace(msg)
	if len(msg) == 0 {
		return nil, 0, errEmptyMessage
	}

	// Most commonly: JSON array.
	if msg[0] == '[' {
		var events []miniTickerEvent
		if err := json.Unmarshal(msg, &events); err != nil {
			return nil, 0, fmt.Errorf("unmarshal miniTicker array: %w", err)
		}
		return events, maxEventTime(events), nil
	}

	// Some clients may use combined streams ({"stream":"...","data":[...]})
	if msg[0] == '{' {
		var env combinedStreamEnvelope
		if err := json.Unmarshal(msg, &env); err != nil {
			return nil, 0, fmt.Errorf("unmarshal ws envelope: %w", err)
		}
		if len(env.Data) == 0 {
			return nil, 0, fmt.Errorf("unexpected ws envelope: missing data")
		}
		var events []miniTickerEvent
		if err := json.Unmarshal(env.Data, &events); err != nil {
			return nil, 0, fmt.Errorf("unmarshal envelope data miniTicker array: %w", err)
		}
		return events, maxEventTime(events), nil
	}

	return nil, 0, fmt.Errorf("unexpected message format (first byte=%q)", msg[0])
}

func maxEventTime(events []miniTickerEvent) int64 {
	var max int64
	for _, e := range events {
		if e.EventTime > max {
			max = e.EventTime
		}
	}
	return max
}

func encodeTickerValue(price string, eventTimeUnixMilli int64) string {
	// Value format: {"price":"<close>","ts":<eventTimeMs>}
	// - price stays as a string to preserve precision.
	b := make([]byte, 0, len(price)+48)
	b = append(b, `{"price":`...)
	b = strconv.AppendQuote(b, price)
	b = append(b, `,"ts":`...)
	b = strconv.AppendInt(b, eventTimeUnixMilli, 10)
	b = append(b, '}')
	return string(b)
}
