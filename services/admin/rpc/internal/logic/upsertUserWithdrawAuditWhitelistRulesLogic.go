package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

type UpsertUserWithdrawAuditWhitelistRulesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpsertUserWithdrawAuditWhitelistRulesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpsertUserWithdrawAuditWhitelistRulesLogic {
	return &UpsertUserWithdrawAuditWhitelistRulesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpsertUserWithdrawAuditWhitelistRulesLogic) UpsertUserWithdrawAuditWhitelistRules(in *pb.UpsertUserWithdrawAuditWhitelistRulesRequest) (*pb.UpsertUserWithdrawAuditWhitelistRulesResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if strings.TrimSpace(in.Reason) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_REASON", "reason required", map[string]string{"reason": "required"})
	}
	if l.svcCtx.DB == nil || l.svcCtx.UserRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}
	if _, err := requireAdminReauth(l.ctx, l.svcCtx, in.Reauth); err != nil {
		return nil, err
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

	if len(in.Rules) > 50 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_RULES", "too many rules", map[string]string{"rules": "too_many"})
	}

	// User self-bound rules are read-only in admin.
	userRuleKeys := map[string]struct{}{}
	{
		ruleRepo := repository.NewUserWithdrawAuditWhitelistRuleRepository(l.svcCtx.DB)
		rows, _ := ruleRepo.ListByUserID(l.ctx, uid)
		for _, rr := range rows {
			if rr == nil {
				continue
			}
			src := strings.ToLower(strings.TrimSpace(rr.Source))
			if src != "user" {
				continue
			}
			k := strings.ToUpper(strings.TrimSpace(rr.AssetCode)) + "|" + strings.ToUpper(strings.TrimSpace(rr.ChainCode)) + "|" + normalizeWhitelistAddress(strings.TrimSpace(rr.Address))
			userRuleKeys[k] = struct{}{}
		}
	}

	type normalizedRule struct {
		AssetCode string
		ChainCode string
		Address   string
		LimitUSDT string
		Enabled   bool
	}

	type pairRef struct {
		Idx       int
		Asset     string
		Chain     string
		Address   string
		LimitUSDT string
	}

	seen := map[string]struct{}{}
	rules := make([]*model.UserWithdrawAuditWhitelistRuleModel, 0, len(in.Rules))
	assetSet := map[string]struct{}{}
	pairs := make([]pairRef, 0, len(in.Rules))
	for idx, r := range in.Rules {
		if r == nil {
			continue
		}
		assetCode := strings.ToUpper(strings.TrimSpace(r.AssetCode))
		chainCode := strings.ToUpper(strings.TrimSpace(r.ChainCode))
		address := normalizeWhitelistAddress(strings.TrimSpace(r.Address))
		src := strings.ToLower(strings.TrimSpace(r.Source))
		if src == "user" {
			return nil, errx.New(
				codes.InvalidArgument,
				400,
				errx.CodeInvalidParam,
				"INVALID_SOURCE",
				"user source rules are read-only",
				map[string]string{fmt.Sprintf("rules.%d.source", idx): "readonly"},
			)
		}
		if src != "" && src != "admin" {
			return nil, errx.New(
				codes.InvalidArgument,
				400,
				errx.CodeInvalidParam,
				"INVALID_SOURCE",
				"invalid source",
				map[string]string{fmt.Sprintf("rules.%d.source", idx): "invalid"},
			)
		}
		if assetCode == "" {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ASSET_CODE", "asset_code required", map[string]string{fmt.Sprintf("rules.%d.asset_code", idx): "required"})
		}
		if chainCode == "" {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_CHAIN_CODE", "chain_code required", map[string]string{fmt.Sprintf("rules.%d.chain_code", idx): "required"})
		}
		if address == "" {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ADDRESS", "address required", map[string]string{fmt.Sprintf("rules.%d.address", idx): "required"})
		}

		limitStr := strings.TrimSpace(r.LimitUsdt)
		if limitStr == "" {
			limitStr = "0"
		}
		limit, err := decimal.NewFromString(limitStr)
		if err != nil || limit.IsNegative() {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_LIMIT_USDT", "invalid limit_usdt", map[string]string{fmt.Sprintf("rules.%d.limit_usdt", idx): "invalid"})
		}
		if limit.Exponent() < -8 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_LIMIT_USDT", "limit_usdt precision too high", map[string]string{fmt.Sprintf("rules.%d.limit_usdt", idx): "precision"})
		}
		limitNormalized := strings.TrimRight(strings.TrimRight(limit.StringFixed(8), "0"), ".")
		if limitNormalized == "" {
			limitNormalized = "0"
		}

		key := assetCode + "|" + chainCode + "|" + address
		if _, ok := seen[key]; ok {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "DUPLICATE_RULE", "duplicate rule", map[string]string{fmt.Sprintf("rules.%d", idx): "duplicate"})
		}
		seen[key] = struct{}{}

		if _, ok := userRuleKeys[key]; ok {
			return nil, errx.New(
				codes.InvalidArgument,
				400,
				errx.CodeInvalidParam,
				"USER_RULE_READONLY",
				"user source rules are read-only",
				map[string]string{fmt.Sprintf("rules.%d", idx): "readonly"},
			)
		}

		assetSet[assetCode] = struct{}{}
		pairs = append(pairs, pairRef{
			Idx:       idx,
			Asset:     assetCode,
			Chain:     chainCode,
			Address:   address,
			LimitUSDT: limitNormalized,
		})

		rules = append(rules, &model.UserWithdrawAuditWhitelistRuleModel{
			UserID:          uid,
			AssetCode:       assetCode,
			ChainCode:       chainCode,
			Address:         address,
			LimitUSDT:       limitNormalized,
			Enabled:         r.Enabled,
			Reason:          strings.TrimSpace(in.Reason),
			OperatorAdminID: current.ID,
			Source:          "admin",
			OperatorUserID:  0,
		})
	}

	// Validate asset-chain pairs: must exist in currency_chain_settings and be withdraw-enabled.
	// This prevents storing rules that can never be matched at runtime.
	if len(pairs) > 0 {
		if l.svcCtx.CurrencyChainSettingsRepo == nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
		}
		allowedByAsset := make(map[string]map[string]struct{}, len(assetSet))
		for asset := range assetSet {
			rows, err := l.svcCtx.CurrencyChainSettingsRepo.ListByAssetCode(l.ctx, asset)
			if err != nil {
				if table, ok := errx.MySQLTableNotFound(err); ok {
					return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
				}
				l.Logger.Errorf("list currency_chain_settings failed: %v", err)
				return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
			}
			allowed := map[string]struct{}{}
			for _, row := range rows {
				if row == nil {
					continue
				}
				// Only allow enabled + withdraw-enabled chains.
				if row.Status != 1 || !row.WithdrawEnabled {
					continue
				}
				cc := strings.ToUpper(strings.TrimSpace(row.ChainCode))
				if cc == "" {
					continue
				}
				allowed[cc] = struct{}{}
			}
			allowedByAsset[asset] = allowed
		}
		for _, p := range pairs {
			if _, ok := allowedByAsset[p.Asset][p.Chain]; !ok {
				return nil, errx.New(
					codes.InvalidArgument,
					400,
					errx.CodeInvalidParam,
					"INVALID_ASSET_CHAIN",
					"asset and chain not matched",
					map[string]string{
						fmt.Sprintf("rules.%d.asset_code", p.Idx): "asset_chain_mismatch",
						fmt.Sprintf("rules.%d.chain_code", p.Idx): "asset_chain_mismatch",
					},
				)
			}
		}
	}

	now := time.Now().UTC()
	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"uid":            uidStr,
		"global_enabled": in.GlobalEnabled,
		"rules":          in.Rules,
		"reason":         strings.TrimSpace(in.Reason),
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		ruleRepo := repository.NewUserWithdrawAuditWhitelistRuleRepository(tx)
		legacyRepo := repository.NewUserWhitelistSettingsRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if _, err := legacyRepo.UpsertByUserID(l.ctx, uid, in.GlobalEnabled, strings.TrimSpace(in.Reason), current.ID); err != nil {
			return err
		}
		if err := ruleRepo.SyncByUserID(l.ctx, uid, rules); err != nil {
			return err
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     current.ID,
			Action:      "user.withdraw_audit_whitelist_rules.upsert",
			TargetType:  "user",
			TargetID:    uidStr,
			Description: "更新用户提现免审白名单规则: " + uidStr,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		if table, ok := errx.MySQLTableNotFound(err); ok {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "DB_SCHEMA_MISSING", "missing table: "+table, nil)
		}
		l.Logger.Errorf("upsert withdraw-audit whitelist rules failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Return latest rules
	ruleRepo := repository.NewUserWithdrawAuditWhitelistRuleRepository(l.svcCtx.DB)
	rows, err := ruleRepo.ListByUserID(l.ctx, uid)
	if err != nil {
		l.Logger.Errorf("list withdraw-audit whitelist rules failed after upsert: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	outRules := make([]*pb.UserWithdrawAuditWhitelistRule, 0, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		outRules = append(outRules, &pb.UserWithdrawAuditWhitelistRule{
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

	return &pb.UpsertUserWithdrawAuditWhitelistRulesResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "WHITELIST_UPDATED"),
		Data: &pb.UpsertUserWithdrawAuditWhitelistRulesData{
			Uid:       uidStr,
			Rules:     outRules,
			UpdatedAt: formatTime(now),
			UpdatedBy: current.Username,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func normalizeWhitelistAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if len(addr) == 42 && (strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X")) {
		hexPart := addr[2:]
		for _, c := range hexPart {
			isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
			if !isHex {
				return addr
			}
		}
		return strings.ToLower(addr)
	}
	return addr
}
