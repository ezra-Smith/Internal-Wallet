package engine

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestComputeRequestHash_IsDeterministic(t *testing.T) {
	p1 := Posting{AssetCode: "USDT", AccountID: 1, Bucket: BalanceBucketAvailable, DebitRaw: decimal.NewFromInt(100), CreditRaw: decimal.Zero}
	p2 := Posting{AssetCode: "USDT", AccountID: 2, Bucket: BalanceBucketAvailable, DebitRaw: decimal.Zero, CreditRaw: decimal.NewFromInt(100)}

	h1, err := computeRequestHash("Transfer", "t1", []Posting{p1, p2})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	h2, err := computeRequestHash("Transfer", "t1", []Posting{p2, p1})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if h1 != h2 {
		t.Fatalf("expected same hash, got %s vs %s", h1, h2)
	}

	h3, err := computeRequestHash("Transfer", "t2", []Posting{p1, p2})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if h1 == h3 {
		t.Fatalf("expected different hash when biz_ref changes")
	}
}
