package scheduler

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

func parseBigInt10(s string) (*big.Int, error) {
	v, ok := new(big.Int).SetString(strings.TrimSpace(s), 10)
	if !ok {
		return nil, fmt.Errorf("invalid integer: %s", s)
	}
	return v, nil
}

func geBigIntString(a string, b string) (bool, error) {
	ai, err := parseBigInt10(a)
	if err != nil {
		return false, err
	}
	bi, err := parseBigInt10(b)
	if err != nil {
		return false, err
	}
	return ai.Cmp(bi) >= 0, nil
}

func addSeconds(now time.Time, seconds int64) time.Time {
	return now.Add(time.Duration(seconds) * time.Second)
}

func addPtrTime(t time.Time) *time.Time {
	return &t
}

func safeMsg(resp any) string {
	if resp == nil {
		return ""
	}
	if mg, ok := resp.(interface{ GetMessage() string }); ok {
		return strings.TrimSpace(mg.GetMessage())
	}
	return ""
}

func retryDelaySeconds(backoffSeconds []int64, attempt int) int64 {
	if attempt <= 0 {
		attempt = 1
	}
	// Default: 1m, 5m, 15m
	if len(backoffSeconds) == 0 {
		backoffSeconds = []int64{60, 300, 900}
	}

	idx := attempt - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(backoffSeconds) {
		idx = len(backoffSeconds) - 1
	}
	sec := backoffSeconds[idx]
	if sec <= 0 {
		sec = 60
	}
	return sec
}

func firstBackoff(backoffSeconds []int64) time.Duration {
	return time.Duration(retryDelaySeconds(backoffSeconds, 1)) * time.Second
}

// mulCeilBigInt multiplies x by multiplier and returns the ceiling integer.
func mulCeilBigInt(x *big.Int, multiplier float64) (*big.Int, error) {
	if x == nil {
		return nil, fmt.Errorf("x is nil")
	}
	if multiplier <= 0 {
		return nil, fmt.Errorf("invalid multiplier: %v", multiplier)
	}
	r := new(big.Rat).SetInt(x)
	m := new(big.Rat)
	if _, ok := m.SetString(fmt.Sprintf("%.6f", multiplier)); !ok {
		return nil, fmt.Errorf("failed to parse multiplier")
	}
	r.Mul(r, m)

	num := r.Num()
	den := r.Denom()
	q := new(big.Int).Quo(num, den)
	rem := new(big.Int).Mod(num, den)
	if rem.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	return q, nil
}
