package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListUser2FAHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUser2FAHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUser2FAHistoryLogic {
	return &ListUser2FAHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListUser2FAHistoryLogic) ListUser2FAHistory(in *pb.ListUser2FAHistoryRequest) (*pb.ListUser2FAHistoryResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.User2FAHistoryRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	uidStr := strings.TrimSpace(in.Uid)
	uid, parseErr := strconv.ParseInt(uidStr, 10, 64)
	if parseErr != nil || uid <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "invalid uid", map[string]string{"uid": "invalid"})
	}

	items, total, err := l.svcCtx.User2FAHistoryRepo.ListByUserID(l.ctx, uid, in.Page, in.PageSize)
	if err != nil {
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "schema not initialized (missing table: "+table+")", nil)
		}
		l.Logger.Errorf("list user 2fa history failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// operator mapping (admin ids -> username)
	adminIDs := make([]int64, 0, len(items))
	seen := map[int64]struct{}{}
	for _, it := range items {
		if it == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(it.OperatorType), "admin") && it.OperatorID > 0 {
			if _, ok := seen[it.OperatorID]; ok {
				continue
			}
			seen[it.OperatorID] = struct{}{}
			adminIDs = append(adminIDs, it.OperatorID)
		}
	}

	adminName := map[int64]string{}
	if len(adminIDs) > 0 {
		type row struct {
			ID       int64
			Username string
		}
		var rows []row
		if qErr := l.svcCtx.DB.WithContext(l.ctx).
			Table("admin_users").
			Select("id, username").
			Where("deleted_at IS NULL AND id IN ?", adminIDs).
			Find(&rows).Error; qErr == nil {
			for _, r := range rows {
				adminName[r.ID] = r.Username
			}
		}
	}

	respItems := make([]*pb.User2FAHistoryItem, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		operator := ""
		switch strings.ToLower(strings.TrimSpace(it.OperatorType)) {
		case "admin":
			operator = adminName[it.OperatorID]
		case "user":
			operator = fmt.Sprintf("%d", it.OperatorID)
		default:
			operator = ""
		}
		respItems = append(respItems, &pb.User2FAHistoryItem{
			Id:           fmt.Sprintf("%d", it.ID),
			Uid:          uidStr,
			Event:        strings.TrimSpace(it.Event),
			Factor:       strings.TrimSpace(it.Factor),
			OperatorType: strings.TrimSpace(it.OperatorType),
			Operator:     operator,
			Reason:       strings.TrimSpace(it.Reason),
			Ip:           strings.TrimSpace(it.IP),
			UserAgent:    strings.TrimSpace(it.UserAgent),
			CreatedAt:    formatTime(it.CreatedAt),
		})
	}

	p := calcPagination(in.Page, in.PageSize, total)
	msg := fmt.Sprintf("ok (%d)", total)
	return &pb.ListUser2FAHistoryResponse{
		Success: true,
		Message: msg,
		Data: &pb.ListUser2FAHistoryData{
			Items: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
