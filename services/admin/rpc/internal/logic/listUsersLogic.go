package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUsersLogic {
	return &ListUsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ==================== Users (Web2) ====================
func (l *ListUsersLogic) ListUsers(in *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	if in == nil {
		in = &pb.ListUsersRequest{}
	}
	if l.svcCtx.UserRepo == nil || l.svcCtx.DB == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	status := strings.TrimSpace(in.Status)
	if status != "" && status != "active" && status != "frozen" && status != "pending_kyc" && status != "terminated" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "无效状态", map[string]string{"status": "invalid"})
	}
	role := strings.TrimSpace(in.Role)
	if role != "" && role != "user" && role != "vip" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ROLE", "无效角色", map[string]string{"role": "invalid"})
	}
	sortBy := strings.TrimSpace(in.SortBy)
	if sortBy != "" && sortBy != "created_at" && sortBy != "last_login" && sortBy != "balance" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_BY", "无效排序字段", map[string]string{"sort_by": "invalid"})
	}
	sortOrder := strings.TrimSpace(in.SortOrder)
	if sortOrder != "" && !strings.EqualFold(sortOrder, "asc") && !strings.EqualFold(sortOrder, "desc") {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_ORDER", "无效排序方向", map[string]string{"sort_order": "invalid"})
	}

	createdFrom, err := parseDateStart(in.CreatedFrom)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "无效日期", map[string]string{"created_from": "invalid"})
	}
	createdTo, err := parseDateEnd(in.CreatedTo)
	if err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_DATE", "无效日期", map[string]string{"created_to": "invalid"})
	}

	var twoFactorEnabled *bool
	if in.TwoFactorEnabled != nil {
		v := in.TwoFactorEnabled.Value
		twoFactorEnabled = &v
	}
	var bypassWithdrawAudit *bool
	if in.BypassWithdrawAudit != nil {
		v := in.BypassWithdrawAudit.Value
		bypassWithdrawAudit = &v
	}

	items, total, err := l.svcCtx.UserRepo.List(
		l.ctx,
		in.Page,
		in.PageSize,
		status,
		role,
		in.Keyword,
		createdFrom,
		createdTo,
		sortBy,
		sortOrder,
		twoFactorEnabled,
		bypassWithdrawAudit,
	)
	if err != nil {
		switch err.Error() {
		case "invalid status":
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "无效状态", map[string]string{"status": "invalid"})
		case "invalid role":
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ROLE", "无效角色", map[string]string{"role": "invalid"})
		case "invalid sort_by":
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_BY", "无效排序字段", map[string]string{"sort_by": "invalid"})
		case "invalid sort_order":
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_SORT_ORDER", "无效排序方向", map[string]string{"sort_order": "invalid"})
		default:
			if table, ok := errx.MySQLTableNotFound(err); ok {
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
			}
			l.Logger.Errorf("list users failed: %v", err)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
	}

	// 2FA status (best-effort: prefer member_security_setting.google_auth_enabled)
	twoFAEnabled := map[int64]bool{}
	googleAuthBound := map[int64]bool{}
	if l.svcCtx.DB != nil && len(items) > 0 {
		ids := make([]int64, 0, len(items))
		seenID := map[int64]struct{}{}
		for _, it := range items {
			if it == nil || it.ID <= 0 {
				continue
			}
			if _, ok := seenID[it.ID]; ok {
				continue
			}
			seenID[it.ID] = struct{}{}
			ids = append(ids, it.ID)
		}
		if len(ids) > 0 {
			type row struct {
				UserID            int64 `gorm:"column:user_id"`
				GoogleAuthEnabled bool  `gorm:"column:google_auth_enabled"`
				GoogleAuthBound   bool  `gorm:"column:google_auth_bound"`
			}
			var rows []row
			if qErr := l.svcCtx.DB.WithContext(l.ctx).
				Table("member_security_setting").
				Select("user_id, google_auth_enabled, google_auth_bound").
				Where("user_id IN ?", ids).
				Find(&rows).Error; qErr == nil {
				for _, r := range rows {
					twoFAEnabled[r.UserID] = r.GoogleAuthEnabled
					googleAuthBound[r.UserID] = r.GoogleAuthBound
				}
			}
		}
	}

	respItems := make([]*pb.UserListItem, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		uidStr := fmt.Sprintf("%d", it.ID)
		twoFactorEnabled := it.Is2faEnabled
		if v, ok := twoFAEnabled[it.ID]; ok {
			twoFactorEnabled = v
		}
		gaaBound := false
		if v, ok := googleAuthBound[it.ID]; ok {
			gaaBound = v
		}
		lastLoginAt := formatTime(it.LastLoginTime)
		lastLoginIP := strings.TrimSpace(it.LastLoginIp)

		created := it.CreatedAt

		statusText := userStatusFrom(it.Status, it.KycLevel)
		freezeAssets := false
		freezeReason := ""

		respItems = append(respItems, &pb.UserListItem{
			Uid:              uidStr,
			Email:            strings.TrimSpace(it.Email),
			Phone:            maskPhone(fullPhone(it.CountryCode, it.Phone)),
			Name:             maskNicknameIfDefault(it.Nickname, it.Phone, it.CountryCode, it.Email),
			Status:           statusText,
			Role:             userRoleFromMemberLevel(it.MemberLevel),
			KycStatus:        userKycStatusFromLevel(it.KycLevel),
			TwoFactorEnabled: twoFactorEnabled,
			TotalBalanceUsd:  "0",
			LastLoginAt:      lastLoginAt,
			LastLoginIp:      lastLoginIP,
			CreatedAt:        formatTime(created),
			FreezeAssets:     freezeAssets,
			FreezeReason:     freezeReason,
			GoogleAuthBound:  gaaBound,
		})
	}

	p := calcPagination(in.Page, in.PageSize, total)
	msg := fmt.Sprintf("ok (%d)", total)
	return &pb.ListUsersResponse{
		Success: true,
		Message: msg,
		Data: &pb.ListUsersData{
			Users: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}

func parseDateStart(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, err
	}
	out := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return &out, nil
}

func parseDateEnd(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, err
	}
	out := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
	return &out, nil
}
