package logic

import (
	"fmt"
	"strings"

	"internalwallet/pkg/accounting"
	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/engine"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/shopspring/decimal"
)

func bucketToString(b pb.BalanceBucket) (string, error) {
	switch b {
	case pb.BalanceBucket_BUCKET_AVAILABLE:
		return engine.BalanceBucketAvailable, nil
	case pb.BalanceBucket_BUCKET_LOCKED:
		return engine.BalanceBucketLocked, nil
	default:
		return "", fmt.Errorf("invalid bucket")
	}
}

func normalSideToString(s pb.NormalSide) (string, error) {
	switch s {
	case pb.NormalSide_NORMAL_SIDE_DEBIT:
		return "debit", nil
	case pb.NormalSide_NORMAL_SIDE_CREDIT:
		return "credit", nil
	default:
		return "", fmt.Errorf("invalid normal_side")
	}
}

func normalSideFromString(s string) pb.NormalSide {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debit":
		return pb.NormalSide_NORMAL_SIDE_DEBIT
	case "credit":
		return pb.NormalSide_NORMAL_SIDE_CREDIT
	default:
		return pb.NormalSide_NORMAL_SIDE_UNSPECIFIED
	}
}

func ownerTypeToString(t pb.OwnerType) (string, error) {
	switch t {
	case pb.OwnerType_OWNER_TYPE_USER:
		return engine.OwnerTypeUser, nil
	case pb.OwnerType_OWNER_TYPE_SYSTEM:
		return engine.OwnerTypeSystem, nil
	default:
		return "", fmt.Errorf("invalid owner_type")
	}
}

func normalizeBizRef(s string) string {
	return strings.TrimSpace(s)
}

func requireDB(svcCtx *svc.ServiceContext) error {
	if svcCtx == nil || svcCtx.DB == nil {
		return fmt.Errorf("db not initialized")
	}
	return nil
}

// parseDecimalToRawRelaxed parses decimal string to raw integer with relaxed precision check.
// Unlike ParseDecimalToRawExact, this function truncates excess decimal places instead of rejecting them.
// This is used for admin manual operations with historical data that may exceed asset precision.
//
// Example: For USDT (precision=6), "100.1234567" will be truncated to "100.123456" (raw: 100123456)
func parseDecimalToRawRelaxed(amountDecimal string, scale int32, allowZero bool) (decimal.Decimal, error) {
	amountDecimal = strings.TrimSpace(amountDecimal)
	if amountDecimal == "" {
		return decimal.Zero, fmt.Errorf("amount required")
	}
	if strings.ContainsAny(amountDecimal, "eE") {
		return decimal.Zero, fmt.Errorf("scientific notation not allowed")
	}
	if scale < 0 || scale > 30 {
		return decimal.Zero, fmt.Errorf("invalid scale")
	}

	amt, err := decimal.NewFromString(amountDecimal)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid amount format")
	}

	if allowZero {
		if amt.LessThan(decimal.Zero) {
			return decimal.Zero, fmt.Errorf("amount must be >= 0")
		}
	} else {
		if amt.LessThanOrEqual(decimal.Zero) {
			return decimal.Zero, fmt.Errorf("amount must be > 0")
		}
	}

	// Truncate to specified precision (instead of rejecting)
	raw := amt.Shift(scale).Truncate(0)

	if err := accounting.ValidateDecimal65Int(raw); err != nil {
		return decimal.Zero, err
	}

	if !allowZero && raw.Equal(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("amount must be > 0")
	}

	return raw, nil
}
