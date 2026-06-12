package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testEnvelope struct {
	Success   bool            `json:"success"`
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"request_id"`
	Timestamp string          `json:"timestamp"`
}

func decodeEnvelope(t *testing.T, rr *httptest.ResponseRecorder) testEnvelope {
	t.Helper()
	var env testEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response: %v, body=%s", err, rr.Body.String())
	}
	return env
}

func TestWriteJSON_PayloadOnlyMap(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-ID", "req-123")

	WriteJSON(rr, map[string]interface{}{"hello": "world"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	if !env.Success || env.Code != 0 {
		t.Fatalf("expected success=true code=0, got success=%v code=%d", env.Success, env.Code)
	}
	if env.RequestID != "req-123" {
		t.Fatalf("expected request_id=req-123, got %q", env.RequestID)
	}
	if env.Timestamp == "" {
		t.Fatalf("expected timestamp to be set")
	}
	if len(env.Data) == 0 {
		t.Fatalf("expected data to be present")
	}
}

func TestWriteJSON_LegacyEnvelope_FailureWithoutCode(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-ID", "req-123")

	WriteJSON(rr, map[string]interface{}{
		"success":    false,
		"message":    "Invalid network_id",
		"request_id": "req-legacy",
		"timestamp":  "2025-01-01T00:00:00Z",
		"data":       map[string]interface{}{"foo": "bar"},
	})

	// Business errors are unified as HTTP 200 with a non-zero business code.
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	if env.Success {
		t.Fatalf("expected success=false")
	}
	if env.Code == 0 {
		t.Fatalf("expected non-zero code")
	}
	if env.RequestID != "req-123" {
		t.Fatalf("expected request_id=req-123 (header), got %q", env.RequestID)
	}
	if env.Message != "Invalid network_id" {
		t.Fatalf("expected message to be preserved, got %q", env.Message)
	}
	// Error responses should keep payload concise.
	if len(env.Data) != 0 {
		t.Fatalf("expected data to be omitted on error, got %s", string(env.Data))
	}
}

func TestWriteJSON_DataWrapper_UnwrapsToAvoidDataDotData(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set("X-Request-ID", "req-123")

	WriteJSON(rr, map[string]interface{}{
		"data": map[string]interface{}{
			"items": []interface{}{map[string]interface{}{"id": 1}},
		},
		"pagination": map[string]interface{}{
			"page": 1,
		},
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	if !env.Success || env.Code != 0 {
		t.Fatalf("expected success=true code=0, got success=%v code=%d", env.Success, env.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("failed to decode data: %v", err)
	}
	if _, ok := payload["data"]; ok {
		t.Fatalf("expected payload to not contain nested data wrapper")
	}
	if _, ok := payload["items"]; !ok {
		t.Fatalf("expected payload to contain items")
	}
	if _, ok := payload["pagination"]; !ok {
		t.Fatalf("expected payload to contain pagination")
	}
}

func TestDynamicRouter_NotFound_IsUnifiedEnvelope(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/unknown", nil)
	rr.Header().Set("X-Request-ID", "req-123")

	r := NewDynamicRouter()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
	env := decodeEnvelope(t, rr)
	if env.Success {
		t.Fatalf("expected success=false")
	}
	if env.Code == 0 {
		t.Fatalf("expected non-zero code")
	}
	if env.RequestID != "req-123" {
		t.Fatalf("expected request_id=req-123, got %q", env.RequestID)
	}
	if env.Timestamp == "" {
		t.Fatalf("expected timestamp to be set")
	}
}

func TestParseQuery_SupportsProtoOptionalScalarPointers(t *testing.T) {
	type req struct {
		MinBalanceUsdRaw *int64 `json:"min_balance_usd_raw,omitempty"`
		NeedsSweep       *bool  `json:"needs_sweep,omitempty"`
		Page             int32  `json:"page,omitempty"`
	}

	r := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/admin/deposit-address-balances?page=1&min_balance_usd_raw=2688700&needs_sweep=true", nil)

	var q req
	if err := ParseQuery(r, &q); err != nil {
		t.Fatalf("expected ParseQuery to succeed, got err=%v", err)
	}
	if q.Page != 1 {
		t.Fatalf("expected page=1, got %d", q.Page)
	}
	if q.MinBalanceUsdRaw == nil || *q.MinBalanceUsdRaw != 2688700 {
		t.Fatalf("expected min_balance_usd_raw=2688700, got %v", q.MinBalanceUsdRaw)
	}
	if q.NeedsSweep == nil || *q.NeedsSweep != true {
		t.Fatalf("expected needs_sweep=true, got %v", q.NeedsSweep)
	}
}
