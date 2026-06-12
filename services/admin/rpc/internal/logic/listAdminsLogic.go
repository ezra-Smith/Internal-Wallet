package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListAdminsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListAdminsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAdminsLogic {
	return &ListAdminsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Admin Users ====================
func (l *ListAdminsLogic) ListAdmins(in *pb.ListAdminsRequest) (*pb.ListAdminsResponse, error) {
	if in == nil {
		in = &pb.ListAdminsRequest{}
	}
	if l.svcCtx.AdminUserRepo == nil || l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	role := strings.TrimSpace(in.Role)
	if role != "" && l.svcCtx.AdminRoleRepo != nil {
		if _, err := l.svcCtx.AdminRoleRepo.FindByCode(l.ctx, role); err != nil {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ROLE", "无效角色", map[string]string{"role": "invalid"})
		}
	}
	status := strings.TrimSpace(in.Status)
	if status != "" && status != "active" && status != "disabled" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "无效状态", map[string]string{"status": "invalid"})
	}

	items, total, err := l.svcCtx.AdminUserRepo.List(l.ctx, in.Page, in.PageSize, role, status, in.Keyword)
	if err != nil {
		l.Logger.Errorf("list admins failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	createdByIDs := make([]int64, 0, len(items))
	uniq := map[int64]struct{}{}
	for _, it := range items {
		if it == nil || it.CreatedBy <= 0 {
			continue
		}
		if _, ok := uniq[it.CreatedBy]; ok {
			continue
		}
		uniq[it.CreatedBy] = struct{}{}
		createdByIDs = append(createdByIDs, it.CreatedBy)
	}

	createdByMap := map[int64]string{}
	if len(createdByIDs) > 0 {
		type row struct {
			ID       int64
			Username string
		}
		var rows []row
		if qErr := l.svcCtx.AdminUserRepo.GetDB().WithContext(l.ctx).
			Model(&model.AdminUserModel{}).
			Select("id, username").
			Where("id IN (?)", createdByIDs).
			Find(&rows).Error; qErr == nil {
			for _, r := range rows {
				createdByMap[r.ID] = r.Username
			}
		}
	}

	respItems := make([]*pb.AdminUserItem, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		createdBy := ""
		if v, ok := createdByMap[it.CreatedBy]; ok {
			createdBy = v
		}
		respItems = append(respItems, &pb.AdminUserItem{
			AdminId:          resp.AdminIDString(it.ID),
			Username:         it.Username,
			Name:             it.Name,
			Role:             it.Role,
			Status:           it.Status,
			TwoFactorEnabled: it.TwoFactorEnabled,
			LastLoginAt:      formatTimePtr(it.LastLoginAt),
			LastLoginIp:      it.LastLoginIP,
			CreatedAt:        formatTime(it.CreatedAt),
			CreatedBy:        createdBy,
		})
	}

	p := calcPagination(in.Page, in.PageSize, total)
	msg := fmt.Sprintf("ok (%d)", total)

	return &pb.ListAdminsResponse{
		Success: true,
		Message: msg,
		Data: &pb.ListAdminsData{
			Admins: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
