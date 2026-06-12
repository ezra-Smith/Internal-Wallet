package logic

import (
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/model"
)

func normalizeAllFilter(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "all") {
		return ""
	}
	return v
}

func normalizeLower(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func validateBlacklistNetwork(network string) bool {
	network = normalizeLower(network)
	if network == "" {
		return false
	}
	if len(network) > 50 {
		return false
	}
	return true
}

func validateBlacklistSource(source string) bool {
	switch normalizeLower(source) {
	case "user_report", "third_party", "auto_detect", "association":
		return true
	default:
		return false
	}
}

func validateBlacklistMonitorStatus(status string) bool {
	switch normalizeLower(status) {
	case "active", "stopped":
		return true
	default:
		return false
	}
}

func toPBBlacklistAddressItem(m *model.BlacklistAddressModel) *pb.BlacklistAddressItem {
	if m == nil {
		return nil
	}
	return &pb.BlacklistAddressItem{
		Id:            m.ID,
		Address:       strings.TrimSpace(m.Address),
		Network:       strings.TrimSpace(m.Network),
		RiskLevel:     strings.TrimSpace(m.RiskLevel),
		Source:        strings.TrimSpace(m.Source),
		HitCount:      m.HitCount,
		LastHitAt:     formatTimePtr(m.LastHitAt),
		MonitorStatus: strings.TrimSpace(m.MonitorStatus),
		Reason:        derefString(m.Reason),
		AddedAt:       formatTimePtr(m.CreatedAt),
		AddedBy:       strings.TrimSpace(m.CreatedBy),
	}
}
