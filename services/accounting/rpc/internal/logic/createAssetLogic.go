package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/common/utils"
	"internalwallet/proto/pb"
	"internalwallet/services/accounting/rpc/internal/model"
	"internalwallet/services/accounting/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAssetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateAssetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAssetLogic {
	return &CreateAssetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateAssetLogic) CreateAsset(in *pb.CreateAssetRequest) (*pb.AcctAssetResponse, error) {
	if err := requireDB(l.svcCtx); err != nil {
		return &pb.AcctAssetResponse{Success: false, Message: err.Error()}, nil
	}
	if l.svcCtx.AssetRepo == nil {
		return &pb.AcctAssetResponse{Success: false, Message: "repository not initialized"}, nil
	}
	if in == nil || strings.TrimSpace(in.Code) == "" {
		return &pb.AcctAssetResponse{Success: false, Message: "code required"}, nil
	}

	code := normalizeAssetCode(in.Code)
	if !isValidAssetCode(code) {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid code"}, nil
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = code
	}
	if len(name) > 64 {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid name"}, nil
	}

	precision := in.Precision
	if precision < 0 || precision > 30 {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid precision"}, nil
	}

	status := in.Status
	if status == 0 {
		status = 2
	}
	if status != 1 && status != 2 {
		return &pb.AcctAssetResponse{Success: false, Message: "invalid status"}, nil
	}

	iconURL := strings.TrimSpace(in.IconUrl)
	if err := validateHTTPIconURL(iconURL); err != nil {
		return &pb.AcctAssetResponse{Success: false, Message: err.Error()}, nil
	}

	m := &model.AssetModel{
		ID:        utils.GenerateID(),
		Code:      code,
		Name:      name,
		IconUrl:   iconURL,
		Precision: precision,
		Status:    int8(status),
	}

	if err := l.svcCtx.AssetRepo.Create(l.ctx, m); err != nil {
		if isMySQLDuplicate(err) {
			return &pb.AcctAssetResponse{Success: false, Message: "asset already exists"}, nil
		}
		l.Logger.Errorf("Create asset failed: %v", err)
		return &pb.AcctAssetResponse{Success: false, Message: "create failed"}, nil
	}

	out, err := l.svcCtx.AssetRepo.FindByCode(l.ctx, code)
	if err != nil {
		l.Logger.Errorf("FindByCode after create failed: %v", err)
		return &pb.AcctAssetResponse{Success: true, Message: fmt.Sprintf("ok (%s)", code)}, nil
	}
	return &pb.AcctAssetResponse{Success: true, Message: "ok", Item: toAcctAssetPB(out)}, nil
}
