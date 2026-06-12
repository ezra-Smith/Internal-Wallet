package logic

import "strings"

func mapCurrencyWithdrawOrderStatusToTxStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "pending", "processing":
		return "pending"
	case "completed":
		return "success"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	default:
		return strings.TrimSpace(status)
	}
}
