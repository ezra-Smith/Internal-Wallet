package logic

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type GetUserDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserDetailLogic {
	return &GetUserDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserDetailLogic) GetUserDetail(in *pb.GetUserDetailRequest) (*pb.GetUserDetailResponse, error) {
	if in == nil || strings.TrimSpace(in.Uid) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "uid required", map[string]string{"uid": "required"})
	}
	if l.svcCtx.UserRepo == nil || l.svcCtx.DB == nil || l.svcCtx.UserAdminNoteRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	uidStr := strings.TrimSpace(in.Uid)
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil || uid <= 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_UID", "invalid uid", map[string]string{"uid": "invalid"})
	}

	u, err := l.svcCtx.UserRepo.FindByID(l.ctx, uid)
	if err != nil || u == nil {
		return nil, errx.New(codes.NotFound, 404, errx.CodeNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	}

	// kyc info (optional)
	var kyc model.UserKycModel
	hasKyc := false
	if qErr := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserKycModel{}).
		Where("user_id = ?", uid).
		First(&kyc).Error; qErr == nil {
		hasKyc = true
	}

	name := strings.TrimSpace(u.Nickname)
	if hasKyc && strings.TrimSpace(kyc.RealName) != "" {
		name = strings.TrimSpace(kyc.RealName)
	}
	// Intelligent masking: if name equals default phone/email, mask it at source.
	name = maskNicknameIfDefault(name, u.Phone, u.CountryCode, u.Email)

	kycStatus := "unverified"
	if u.KycLevel > 0 {
		kycStatus = "verified"
	} else if hasKyc {
		kycStatus = "pending"
	}

	status := ""
	switch u.Status {
	case 2:
		status = "frozen"
	case 3:
		status = "terminated"
	default:
		if kycStatus == "pending" {
			status = "pending_kyc"
		} else {
			status = "active"
		}
	}

	role := userRoleFromMemberLevel(u.MemberLevel)

	created := u.CreatedAt

	// last login (optional)
	var lastLogin model.UserLoginRecordModel
	var lastLoginAt time.Time
	lastLoginIP := ""
	lastLoginDevice := ""
	if qErr := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserLoginRecordModel{}).
		Where("user_id = ? AND result = ?", uid, "success").
		Order("login_time DESC").
		First(&lastLogin).Error; qErr == nil {
		lastLoginAt = lastLogin.LoginTime
		lastLoginIP = strings.TrimSpace(lastLogin.IpAddress)
		lastLoginDevice = strings.TrimSpace(lastLogin.DeviceName)
	} else {
		lastLoginAt = u.LastLoginTime
		lastLoginIP = strings.TrimSpace(u.LastLoginIp)
	}

	// login attempts today (best-effort)
	loginAttemptsToday := int32(0)
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var cnt int64
	if qErr := l.svcCtx.DB.WithContext(l.ctx).
		Model(&model.UserLoginRecordModel{}).
		Where("user_id = ? AND login_time >= ?", uid, startOfDay).
		Count(&cnt).Error; qErr == nil {
		loginAttemptsToday = int32(cnt)
	}

	// admin notes
	notes, nErr := l.svcCtx.UserAdminNoteRepo.ListByUserID(l.ctx, uid, 50)
	if nErr != nil {
		l.Logger.Errorf("list user notes failed: %v", nErr)
		notes = nil
	}

	operatorIDs := make([]int64, 0, len(notes))
	seen := map[int64]struct{}{}
	for _, n := range notes {
		if n == nil || n.OperatorAdminID <= 0 {
			continue
		}
		if _, ok := seen[n.OperatorAdminID]; ok {
			continue
		}
		seen[n.OperatorAdminID] = struct{}{}
		operatorIDs = append(operatorIDs, n.OperatorAdminID)
	}

	operatorMap := map[int64]string{}
	if len(operatorIDs) > 0 {
		type row struct {
			ID       int64
			Username string
		}
		var rows []row
		if qErr := l.svcCtx.DB.WithContext(l.ctx).
			Table("admin_users").
			Select("id, username").
			Where("deleted_at IS NULL AND id IN ?", operatorIDs).
			Find(&rows).Error; qErr == nil {
			for _, r := range rows {
				operatorMap[r.ID] = r.Username
			}
		}
	}

	respNotes := make([]*pb.UserAdminNote, 0, len(notes))
	for _, n := range notes {
		if n == nil {
			continue
		}
		operator := ""
		if v, ok := operatorMap[n.OperatorAdminID]; ok {
			operator = v
		}
		respNotes = append(respNotes, &pb.UserAdminNote{
			Id:          fmt.Sprintf("%d", n.ID),
			Uid:         uidStr,
			Content:     n.Content,
			IsImportant: n.IsImportant,
			Operator:    operator,
			CreatedAt:   formatTime(n.CreatedAt),
		})
	}

	// 2FA status (best-effort: prefer member_security_setting.google_auth_enabled)
	twoFactorEnabled := u.Is2faEnabled
	var googleAuthBound bool
	if l.svcCtx.DB != nil {
		type row struct {
			GoogleAuthEnabled bool `gorm:"column:google_auth_enabled"`
			GoogleAuthBound   bool `gorm:"column:google_auth_bound"`
		}
		var r row
		if qErr := l.svcCtx.DB.WithContext(l.ctx).
			Table("member_security_setting").
			Select("google_auth_enabled, google_auth_bound").
			Where("user_id = ?", uid).
			Take(&r).Error; qErr == nil {
			twoFactorEnabled = r.GoogleAuthEnabled
			googleAuthBound = r.GoogleAuthBound
		}
	}
	securityType := ""
	if twoFactorEnabled {
		securityType = "totp"
	}

	// withdraw audit bypass whitelist (best-effort)
	bypassWithdrawAudit := false
	// Prefer new per-rule model; fallback to legacy global flag.
	if l.svcCtx.DB != nil {
		var n int64
		if qErr := l.svcCtx.DB.WithContext(l.ctx).
			Table("user_withdraw_audit_whitelist_rules").
			Where("deleted_at IS NULL AND enabled = 1 AND user_id = ?", uid).
			Count(&n).Error; qErr == nil && n > 0 {
			bypassWithdrawAudit = true
		}
	}
	if !bypassWithdrawAudit && l.svcCtx.UserWhitelistSettingsRepo != nil {
		if s, wErr := l.svcCtx.UserWhitelistSettingsRepo.GetByUserID(l.ctx, uid); wErr == nil && s != nil && s.BypassWithdrawAudit {
			bypassWithdrawAudit = true
		}
	}

	return &pb.GetUserDetailResponse{
		Success: true,
		Message: "ok",
		Data: &pb.GetUserDetailData{
			BasicInfo: &pb.UserBasicInfo{
				Uid:       uidStr,
				Email:     strings.TrimSpace(u.Email),
				Phone:     fullPhone(u.CountryCode, u.Phone),
				Name:      name,
				Avatar:    strings.TrimSpace(u.Avatar),
				Status:    status,
				Role:      role,
				CreatedAt: formatTime(created),
			},
			SecurityInfo: &pb.UserSecurityInfo{
				TwoFactorEnabled:   twoFactorEnabled,
				TwoFactorType:      securityType,
				LastPasswordChange: "",
				LoginAttemptsToday: loginAttemptsToday,
				LastLoginAt:        formatTime(lastLoginAt),
				LastLoginIp:        lastLoginIP,
				LastLoginDevice:    lastLoginDevice,
				GoogleAuthBound:    googleAuthBound,
			},
			KycInfo: &pb.UserKycInfo{
				Status:     kycStatus,
				Level:      u.KycLevel,
				VerifiedAt: "",
				IdType:     "",
				IdNumber:   maskIDNumber(kyc.IdNumber),
			},
			AssetSummary: &pb.UserAssetSummary{
				TotalBalanceUsd: "0",
				Balances:        []*pb.UserAssetBalance{},
			},
			TransactionSummary: &pb.UserTransactionSummary{
				TotalDeposits:            0,
				TotalDepositAmountUsd:    "0",
				TotalWithdrawals:         0,
				TotalWithdrawalAmountUsd: "0",
				TotalSwaps:               0,
				LastTransactionAt:        "",
			},
			WhitelistAddresses:  []*pb.UserWhitelistAddress{},
			AdminNotes:          respNotes,
			BypassWithdrawAudit: bypassWithdrawAudit,
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
