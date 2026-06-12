package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListUserWithdrawAuditWhitelistRulesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListUserWithdrawAuditWhitelistRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListUserWithdrawAuditWhitelistRulesLogic {
	return &ListUserWithdrawAuditWhitelistRulesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListUserWithdrawAuditWhitelistRulesLogic) ListUserWithdrawAuditWhitelistRules(in *pb.ListUserWithdrawAuditWhitelistRulesRequest) (*pb.ListUserWithdrawAuditWhitelistRulesResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	uidStr := strings.TrimSpace(in.Uid)
	uid, parseErr := strconv.ParseInt(uidStr, 10, 64)
	if parseErr != nil || uid <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "invalid uid", map[string]string{"uid": "invalid"})
	}

	u, err := l.svcCtx.UserRepo.FindByID(l.ctx, uid)
	if err != nil || u == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	}

	ruleRepo := repository.NewUserWithdrawAuditWhitelistRuleRepository(l.svcCtx.DB)
	rows, err := ruleRepo.ListByUserID(l.ctx, uid)
	if err != nil {
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
		}
		l.Logger.Errorf("list withdraw-audit whitelist rules failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	respRules := make([]*pb.UserWithdrawAuditWhitelistRule, 0, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		respRules = append(respRules, &pb.UserWithdrawAuditWhitelistRule{
			Id:        fmt.Sprintf("%d", r.ID),
			Uid:       uidStr,
			AssetCode: strings.TrimSpace(r.AssetCode),
			ChainCode: strings.TrimSpace(r.ChainCode),
			Address:   strings.TrimSpace(r.Address),
			LimitUsdt: strings.TrimSpace(r.LimitUSDT),
			Enabled:   r.Enabled,
			UpdatedAt: formatTime(r.UpdatedAt),
			Source:    strings.TrimSpace(r.Source),
		})
	}

	globalEnabled := false
	if l.svcCtx.UserWhitelistSettingsRepo != nil {
		if s, wErr := l.svcCtx.UserWhitelistSettingsRepo.GetByUserID(l.ctx, uid); wErr == nil && s != nil && s.BypassWithdrawAudit {
			globalEnabled = true
		}
	}

	return &pb.ListUserWithdrawAuditWhitelistRulesResponse{
		Success: true,
		Message: "ok",
		Data: &pb.ListUserWithdrawAuditWhitelistRulesData{
			GlobalEnabled: globalEnabled,
			Rules:         respRules,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
