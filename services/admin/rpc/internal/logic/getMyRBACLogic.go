package logic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/rbac"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetMyRBACLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMyRBACLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMyRBACLogic {
	return &GetMyRBACLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetMyRBACLogic) GetMyRBAC(in *pb.GetMyRBACRequest) (*pb.GetMyRBACResponse, error) {
	_ = in
	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if l.svcCtx.AdminRBACRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "rbac not ready", nil)
	}

	adminID := current.ID
	roleForResp := current.Role

	menus, err := l.svcCtx.AdminRBACRepo.GetUserMenus(l.ctx, adminID)
	if err != nil {
		l.Errorw("query user menus failed", logx.Field("error", err))
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING",
				fmt.Sprintf("rbac schema not initialized (missing table: %s)", table), nil)
		}
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query user menus failed", nil)
	}
	menus, _ = dedupeMenuModels(menus)
	userMenus := make([]*pb.AdminMenu, 0, len(menus))
	for _, m := range menus {
		userMenus = append(userMenus, toPBAdminMenu(m))
	}
	userMenuTree := buildMenuTreeFromModels(menus, 0, true, true)

	perms2, err := l.svcCtx.AdminRBACRepo.GetUserPermissions(l.ctx, adminID)
	if err != nil {
		l.Errorw("query user permissions failed", logx.Field("error", err))
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING",
				fmt.Sprintf("rbac schema not initialized (missing table: %s)", table), nil)
		}
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query user permissions failed", nil)
	}
	permByCode := map[string]*pb.AdminPermission{}
	for _, p := range perms2 {
		pp := toPBAdminPermission(p)
		if pp == nil || strings.TrimSpace(pp.Code) == "" {
			continue
		}
		if _, ok := permByCode[pp.Code]; ok {
			continue
		}
		permByCode[pp.Code] = pp
	}
	dbPermissionCodes := make([]string, 0, len(permByCode))
	for code := range permByCode {
		dbPermissionCodes = append(dbPermissionCodes, code)
	}
	sort.Strings(dbPermissionCodes)
	userPermissions := make([]*pb.AdminPermission, 0, len(dbPermissionCodes))
	for _, code := range dbPermissionCodes {
		userPermissions = append(userPermissions, permByCode[code])
	}

	roles, err := l.svcCtx.AdminRBACRepo.GetUserRoles(l.ctx, adminID)
	if err != nil {
		l.Errorw("query user roles failed", logx.Field("error", err))
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING",
				fmt.Sprintf("rbac schema not initialized (missing table: %s)", table), nil)
		}
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "query user roles failed", nil)
	}
	if len(roles) > 0 && strings.TrimSpace(roles[0].Code) != "" {
		roleForResp = roles[0].Code
	}
	userRoles := make([]*pb.AdminRole, 0, len(roles))
	for _, r := range roles {
		userRoles = append(userRoles, toPBAdminRole(r))
	}

	userPermissionCodes := rbac.CanonicalizePermissionCodes(dbPermissionCodes)
	tree, buildErr := rbac.BuildPermissionsTree(userPermissionCodes)
	if buildErr != nil {
		l.Errorw("permissions build failed", logx.Field("error", buildErr))
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "permissions build failed", nil)
	}

	return &pb.GetMyRBACResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetMyRBACData{
			AdminId:             resp.AdminIDString(adminID),
			Role:                roleForResp,
			Permissions:         tree,
			UserMenus:           userMenus,
			UserMenuTree:        userMenuTree,
			UserPermissions:     userPermissions,
			UserPermissionCodes: userPermissionCodes,
			UserRoles:           userRoles,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
