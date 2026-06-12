package logic

import (
	"encoding/json"
	"testing"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
)

func TestDeriveWeb3TxStatusFromChain(t *testing.T) {
	t.Run("pending stays pending", func(t *testing.T) {
		got := deriveWeb3TxStatusFromChain(pb.TxStatus_TX_STATUS_PENDING, 0, 12)
		if got != model.Web3TxStatusPending {
			t.Fatalf("expected %q, got %q", model.Web3TxStatusPending, got)
		}
	})

	t.Run("confirmed but insufficient confirmations -> pending", func(t *testing.T) {
		got := deriveWeb3TxStatusFromChain(pb.TxStatus_TX_STATUS_CONFIRMED, 5, 12)
		if got != model.Web3TxStatusPending {
			t.Fatalf("expected %q, got %q", model.Web3TxStatusPending, got)
		}
	})

	t.Run("confirmed and enough confirmations -> confirmed", func(t *testing.T) {
		got := deriveWeb3TxStatusFromChain(pb.TxStatus_TX_STATUS_CONFIRMED, 12, 12)
		if got != model.Web3TxStatusConfirmed {
			t.Fatalf("expected %q, got %q", model.Web3TxStatusConfirmed, got)
		}
	})

	t.Run("failed is failed", func(t *testing.T) {
		got := deriveWeb3TxStatusFromChain(pb.TxStatus_TX_STATUS_FAILED, 0, 12)
		if got != model.Web3TxStatusFailed {
			t.Fatalf("expected %q, got %q", model.Web3TxStatusFailed, got)
		}
	})
}

func TestIsNotFoundMessage(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"transaction not found", true},
		{"NOT FOUND", true},
		{"未找到交易", true},
		{"temporary error", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := isNotFoundMessage(tc.msg); got != tc.want {
			t.Fatalf("isNotFoundMessage(%q) expected %v, got %v", tc.msg, tc.want, got)
		}
	}
}

func TestMergeRawData(t *testing.T) {
	patch := map[string]interface{}{
		"reconcile": map[string]interface{}{
			"reason": "not_found",
		},
	}

	b := mergeRawData(nil, patch)
	if len(b) == 0 {
		t.Fatalf("expected non-empty json")
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, ok := m["reconcile"]; !ok {
		t.Fatalf("expected reconcile key in merged json")
	}

	// Ensure shallow merge works for nested objects.
	b2 := mergeRawData(b, map[string]interface{}{
		"reconcile": map[string]interface{}{
			"balance_synced_at": 123,
		},
	})
	var m2 map[string]interface{}
	if err := json.Unmarshal(b2, &m2); err != nil {
		t.Fatalf("unmarshal2 failed: %v", err)
	}
	rec, ok := m2["reconcile"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected reconcile object")
	}
	if rec["reason"] != "not_found" {
		t.Fatalf("expected reconcile.reason preserved, got %v", rec["reason"])
	}
	if rec["balance_synced_at"] == nil {
		t.Fatalf("expected reconcile.balance_synced_at set")
	}
}
