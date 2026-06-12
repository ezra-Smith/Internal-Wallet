package logic

import (
	"encoding/json"
	"strings"

	"internalwallet/common/constants"
	"internalwallet/services/admin/rpc/internal/model"
)

func newWithdrawOrderEvent(withdrawOrderID int64, eventType, actorType string, actorID int64, ip string, summary string, details any) *model.CurrencyWithdrawOrderEventModel {
	eventType = strings.TrimSpace(eventType)
	actorType = strings.TrimSpace(actorType)
	summary = strings.TrimSpace(summary)

	var ipPtr *string
	if actorType == constants.WithdrawActorAdmin {
		v := strings.TrimSpace(ip)
		if v != "" {
			ipPtr = &v
		}
	}

	var detailsBytes []byte
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailsBytes = b
		}
	}
	if summary == "" {
		summary = eventType
	}
	return &model.CurrencyWithdrawOrderEventModel{
		WithdrawOrderID: withdrawOrderID,
		EventType:       eventType,
		ActorType:       actorType,
		ActorID:         actorID,
		IP:              ipPtr,
		Summary:         summary,
		Details:         detailsBytes,
	}
}
