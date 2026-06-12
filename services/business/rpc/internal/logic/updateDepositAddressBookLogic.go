package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateDepositAddressBookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateDepositAddressBookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateDepositAddressBookLogic {
	return &UpdateDepositAddressBookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateDepositAddressBookLogic) UpdateDepositAddressBook(in *pb.UpdateDepositAddressBookReq) (*pb.UpdateDepositAddressBookResp, error) {
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	id, err := strconv.ParseInt(strings.TrimSpace(in.Id), 10, 64)
	if err != nil || id <= 0 {
		return nil, errx.InvalidParam("invalid params")
	}

	if l.svcCtx.DepositAddressBookRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	if err := l.svcCtx.DepositAddressBookRepository.UpdateLabelRemark(l.ctx, uid, id, trimToNil(in.Label), trimToNil(in.Remark)); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return nil, errx.NotFound("deposit address not found")
		}
		l.Errorf("UpdateLabelRemark failed: %v", err)
		return nil, errx.Internal("update deposit address failed")
	}

	return &pb.UpdateDepositAddressBookResp{
		Success: true,
		Message: "ok",
	}, nil
}
