package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAccountTypeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateAccountTypeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAccountTypeLogic {
	return &UpdateAccountTypeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateAccountTypeLogic) UpdateAccountType(in *pb.UpdateAccountTypeRequest) (*pb.AcctAccountTypeResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.AcctAccountTypeResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AccountTypeRepo == nil {
		return &pb.AcctAccountTypeResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil {
		return &pb.AcctAccountTypeResponse{Success: false, Message: "invalid request"}, nil
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if code == "" {
		return &pb.AcctAccountTypeResponse{Success: false, Message: "code required"}, nil
	}
	fields := map[string]interface{}{}
	if strings.TrimSpace(in.Name) != "" {
		fields["name"] = strings.TrimSpace(in.Name)
	}

	fields["description"] = strings.TrimSpace(in.Description)

	if in.NormalSide != pb.NormalSide_NORMAL_SIDE_UNSPECIFIED {
		ns, err := normalSideToString(in.NormalSide)
		if err != nil {
			return &pb.AcctAccountTypeResponse{Success: false, Message: err.Error()}, nil
		}
		fields["normal_side"] = ns
	}
	if err := l.svcCtx.AccountTypeRepo.Update(l.ctx, code, fields); err != nil {
		l.Logger.Errorf("UpdateAccountType failed: %v", err)
		return &pb.AcctAccountTypeResponse{Success: false, Message: err.Error()}, nil
	}
	// Return latest.
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
