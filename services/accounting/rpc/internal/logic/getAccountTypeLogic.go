package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetAccountTypeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetAccountTypeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAccountTypeLogic {
	return &GetAccountTypeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetAccountTypeLogic) GetAccountType(in *pb.GetAccountTypeRequest) (*pb.AcctAccountTypeResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.AcctAccountTypeResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AccountTypeRepo == nil {
		return &pb.AcctAccountTypeResponse{Success: false, Message: "repository not initialized"}, nil
	}
	code := ""
	if in != nil {
		code = strings.ToUpper(strings.TrimSpace(in.Code))
	}
	if code == "" {
		return &pb.AcctAccountTypeResponse{Success: false, Message: "code required"}, nil
	}
	m, err := l.svcCtx.AccountTypeRepo.FindByCode(l.ctx, code, false)
	if err != nil {
		return &pb.AcctAccountTypeResponse{Success: false, Message: err.Error()}, nil
	}
	assets, _ := l.svcCtx.AccountTypeRepo.ListAssets(l.ctx, code)
	return &pb.AcctAccountTypeResponse{
		Success: true,
		Message: "ok",
		Item: &pb.AcctAccountType{
			Code:        m.Code,
			Name:        m.Name,
			Description: m.Description,
			NormalSide:  normalSideFromString(m.NormalSide),
			AssetCodes:  assets,
		},
	}, nil
}
