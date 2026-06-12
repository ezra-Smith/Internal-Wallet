package logic

import (
	"context"
	"encoding/json"
	"strings"

	"internalwallet/common/constants"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"

	"github.com/zeromicro/go-zero/core/logx"
)

func newWithdrawOrderEvent(withdrawOrderID int64, eventType, actorType string, actorID int64, ip *string, summary string, details any) *model.CurrencyWithdrawOrderEventModel {
	eventType = strings.TrimSpace(eventType)
	actorType = strings.TrimSpace(actorType)
	summary = strings.TrimSpace(summary)
	if actorType != constants.WithdrawActorAdmin {
		ip = nil
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
		IP:              ip,
		Summary:         summary,
		Details:         detailsBytes,
	}
}

func appendWithdrawOrderEvent(ctx context.Context, repo repository.CurrencyWithdrawOrderEventRepository, e *model.CurrencyWithdrawOrderEventModel) {
	if repo == nil || e == nil {
		return
	}
	if err := repo.Create(ctx, e); err != nil {
		logx.WithContext(ctx).Errorf("append withdraw order event failed: %v", err)
	}
}

func appendWithdrawOrderEvents(ctx context.Context, repo repository.CurrencyWithdrawOrderEventRepository, events []*model.CurrencyWithdrawOrderEventModel) {
	if repo == nil || len(events) == 0 {
		return
	}
	if err := repo.CreateMultiple(ctx, events); err != nil {
		logx.WithContext(ctx).Errorf("append withdraw order events failed: %v", err)
	}
}
