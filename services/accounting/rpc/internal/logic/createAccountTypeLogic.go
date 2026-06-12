package logic

import (
	"context"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/model"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAccountTypeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateAccountTypeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAccountTypeLogic {
	return &CreateAccountTypeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Account Types ====================
func (l *CreateAccountTypeLogic) CreateAccountType(in *pb.CreateAccountTypeRequest) (*pb.AcctAccountTypeResponse, error) {
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
	name := strings.TrimSpace(in.Name)
	description := strings.TrimSpace(in.Description)
	normalSide, err := normalSideToString(in.NormalSide)
	if err != nil {
		return &pb.AcctAccountTypeResponse{Success: false, Message: err.Error()}, nil
	}
	if code == "" {
		return &pb.AcctAccountTypeResponse{Success: false, Message: "code required"}, nil
	}
	m := &model.AcctAccountTypeModel{
		Code:        code,
		Name:        name,
		Description: description,
		NormalSide:  normalSide,
	}
	if err := l.svcCtx.AccountTypeRepo.Create(l.ctx, m); err != nil {
		l.Logger.Errorf("CreateAccountType failed: %v", err)
		return &pb.AcctAccountTypeResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.AcctAccountTypeResponse{
		Success: true,
		Message: "ok",
		Item: &pb.AcctAccountType{
			Code:        m.Code,
			Name:        m.Name,
			Description: m.Description,
			NormalSide:  normalSideFromString(m.NormalSide),
		},
	}, nil
}
