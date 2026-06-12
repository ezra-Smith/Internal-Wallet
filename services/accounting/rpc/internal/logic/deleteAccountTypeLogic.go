package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAccountTypeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteAccountTypeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAccountTypeLogic {
	return &DeleteAccountTypeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteAccountTypeLogic) DeleteAccountType(in *pb.DeleteAccountTypeRequest) (*pb.Empty, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.Empty{}, nil
	}
	if l.svcCtx.AccountTypeRepo == nil {
		return &pb.Empty{}, nil
	}
	code := ""
	if in != nil {
		code = strings.ToUpper(strings.TrimSpace(in.Code))
	}
	if code == "" {
		return &pb.Empty{}, nil
	}
	if err := l.svcCtx.AccountTypeRepo.SoftDelete(l.ctx, code); err != nil {
		l.Logger.Errorf("DeleteAccountType failed: %v", err)
	}
	return &pb.Empty{}, nil
}
