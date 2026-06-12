package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SetDefaultDepositAddressBookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSetDefaultDepositAddressBookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SetDefaultDepositAddressBookLogic {
	return &SetDefaultDepositAddressBookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SetDefaultDepositAddressBookLogic) SetDefaultDepositAddressBook(in *pb.SetDefaultDepositAddressBookReq) (*pb.SetDefaultDepositAddressBookResp, error) {
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, err := strconv.ParseInt(strings.TrimSpace(uidStr), 10, 64)
	if err != nil || uid <= 0 {
		return nil, errx.Unauthorized("unauthorized")
	}

	id, err := strconv.ParseInt(strings.TrimSpace(in.Id), 10, 64)
	if err != nil || id <= 0 {
		return nil, errx.InvalidParam("invalid params")
	}
	if l.svcCtx.DepositAddressBookRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	addr, err := l.svcCtx.DepositAddressBookRepository.FindActiveByID(l.ctx, uid, id)
	if err != nil || addr == nil {
		if errors.Is(err, repository.ErrDepositAddressNotFound) {
			return nil, errx.NotFound("deposit address not found")
		}
		l.Errorf("FindActiveByID failed: uid=%d id=%d err=%v", uid, id, err)
		return nil, errx.Internal("query deposit address failed")
	}

	chainCode := strings.TrimSpace(addr.ChainCode)
	if err := l.svcCtx.DepositAddressBookRepository.SetDefaultByID(l.ctx, uid, chainCode, id); err != nil {
		l.Errorf("SetDefaultByID failed: %v", err)
		if errors.Is(err, repository.ErrDepositAddressNotFound) {
			return nil, errx.NotFound("deposit address not found")
		}
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errx.Conflict("duplicate default")
		}
		return nil, errx.Internal("set default failed")
	}

	return &pb.SetDefaultDepositAddressBookResp{
		Success: true,
		Message: "ok",
	}, nil
}
