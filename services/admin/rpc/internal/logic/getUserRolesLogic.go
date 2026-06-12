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

type GetUserRolesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserRolesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserRolesLogic {
	return &GetUserRolesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserRolesLogic) GetUserRoles(in *pb.GetUserRolesRequest) (*pb.GetUserRolesResponse, error) {
	if in == nil || in.UserId <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid user_id", map[string]string{"user_id": "required"})
	}
	if l.svcCtx.AdminUserRepo == nil || l.svcCtx.AdminRBACRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	if _, err := l.svcCtx.AdminUserRepo.FindByID(l.ctx, in.UserId); err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "user not found", nil)
	}

	roles, err := l.svcCtx.AdminRBACRepo.GetUserRoles(l.ctx, in.UserId)
	if err != nil {
		l.Logger.Errorf("get user roles failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query failed", nil)
	}
	out := make([]*pb.AdminRole, 0, len(roles))
	for _, r := range roles {
		out = append(out, toPBAdminRole(r))
	}

	return &pb.GetUserRolesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetUserRolesData{
			List: out,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
