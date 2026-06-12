package logic

import (
	"context"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type DeleteMenuLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteMenuLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMenuLogic {
	return &DeleteMenuLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteMenuLogic) DeleteMenu(in *pb.DeleteMenuRequest) (*pb.DeleteMenuResponse, error) {
	if in == nil || in.Id <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid id", map[string]string{"id": "required"})
	}
	if l.svcCtx.AdminMenuRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "service not ready", nil)
	}

	_, err := l.svcCtx.AdminMenuRepo.FindByID(l.ctx, in.Id)
	if err != nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "NOT_FOUND", "menu not found", nil)
	}

	cnt, err := l.svcCtx.AdminMenuRepo.CountActiveChildren(l.ctx, in.Id)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "check children failed", nil)
	}
	if cnt > 0 {
		return nil, errx.New(codes.FailedPrecondition, 400, errx.CodeInvalidParam, "CANNOT_DELETE", "Cannot delete menu with active children", nil)
	}

	now := time.Now()
	if err := l.svcCtx.AdminMenuRepo.SoftDelete(l.ctx, in.Id, now); err != nil {
		l.Logger.Errorf("delete menu failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "delete menu failed", nil)
	}

	return &pb.DeleteMenuResponse{
		Success:   true,
		Message:   "ok",
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
