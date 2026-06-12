package logic

import (
	"context"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetPermissionByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPermissionByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPermissionByIdLogic {
	return &GetPermissionByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPermissionByIdLogic) GetPermissionById(in *pb.GetPermissionByIdRequest) (*pb.GetPermissionByIdResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.AdminPermissionRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	m, err := l.svcCtx.AdminPermissionRepo.FindByID(l.ctx, in.Id)
	if err != nil || m == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "permission not found", nil)
	}

	return &pb.GetPermissionByIdResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetPermissionByIdData{
			Permission: toPBAdminPermission(m),
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
