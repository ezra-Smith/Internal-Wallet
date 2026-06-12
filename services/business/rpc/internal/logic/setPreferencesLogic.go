package logic

import (
	"context"
	"strconv"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetPreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetPreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetPreferencesLogic {
	return &SetPreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetPreferencesLogic) SetPreferences(in *pb.SetPreferencesReq) (*pb.SetPreferencesResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	id, _ := strconv.ParseInt(uidStr, 10, 64)
	accountRepo := l.svcCtx.UserAccountRepository

	// 验证并转换 price_unit
	priceUnit := in.PriceUnit
	if priceUnit == pb.PriceUnit_PRICE_UNIT_UNSPECIFIED {
		// 如果前端传了无效值，保持原值不变（可能是 0，表示不更新）
		l.Infof("price_unit is unspecified, skipping update")
	} else if priceUnit < 0 || priceUnit > 3 {
		return nil, errx.InvalidParam("invalid price_unit value")
	}
	
	// 验证并转换 language
	language := in.Language
	if language < 0 || language > 2 {
		return nil, errx.InvalidParam("invalid language value")
	}

	_ = accountRepo.UpdatePreferences(l.ctx, id, int(language), int(priceUnit))
	return &pb.SetPreferencesResp{Success: true, Message: "ok"}, nil
}
