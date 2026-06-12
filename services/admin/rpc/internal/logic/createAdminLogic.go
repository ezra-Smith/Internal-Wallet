package logic

import (
	"context"
	"encoding/json"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/repository"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreateAdminLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateAdminLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAdminLogic {
	return &CreateAdminLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateAdminLogic) CreateAdmin(in *pb.CreateAdminRequest) (*pb.CreateAdminResponse, error) {
	violations := make(map[string]string, 3)
	if in == nil || strings.TrimSpace(in.Username) == "" {
		violations["username"] = "required"
	}
	if in == nil || strings.TrimSpace(in.Name) == "" {
		violations["name"] = "required"
	}
	if in == nil || strings.TrimSpace(in.Role) == "" {
		violations["role"] = "required"
	}
	if len(violations) > 0 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", violations)
	}
	if l.svcCtx.DB == nil || l.svcCtx.AdminUserRepo == nil || l.svcCtx.PasswordHistoryRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	username := strings.TrimSpace(in.Username)
	if _, err := mail.ParseAddress(username); err != nil {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_EMAIL", "邮箱格式无效", map[string]string{"username": "invalid email"})
	}

	role := strings.TrimSpace(in.Role)
	if role == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ROLE", "无效角色", map[string]string{"role": "required"})
	}
	if l.svcCtx.AdminRoleRepo == nil || l.svcCtx.AdminUserRoleRepo == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "rbac not ready", nil)
	}
	roleModel, err := l.svcCtx.AdminRoleRepo.FindByCode(l.ctx, role)
	if err != nil || roleModel == nil || roleModel.Status != 1 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ROLE", "无效角色", map[string]string{"role": "invalid"})
	}

	minLen := int(l.svcCtx.Config.Security.PasswordMinLength)
	if minLen <= 0 {
		minLen = 10
	}

	plainPassword := in.Password
	tempPassword := ""
	if plainPassword == "" {
		p, genErr := security.GenerateTempPassword(minLen + 2)
		if genErr != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "generate password failed", nil)
		}
		if err := security.ValidatePasswordPolicy(p, minLen); err != nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "generate password failed", nil)
		}
		plainPassword = p
		tempPassword = p
	}

	if err := security.ValidatePasswordPolicy(plainPassword, minLen); err != nil {
		return nil, errx.NewWithMetadata(codes.InvalidArgument, 422, errx.CodePasswordTooWeak, "PASSWORD_TOO_WEAK", err.Error(), map[string]string{"password": "too weak"}, map[string]string{
			"min_length": strconv.Itoa(minLen),
		})
	}

	// 不能包含用户名（邮箱）/ local part
	passLower := strings.ToLower(plainPassword)
	userLower := strings.ToLower(username)
	if userLower != "" && strings.Contains(passLower, userLower) {
		return nil, errx.New(codes.InvalidArgument, 422, errx.CodePasswordTooWeak, "PASSWORD_CONTAINS_USERNAME", "password must not contain username", map[string]string{"password": "contains username"})
	}
	if at := strings.Index(userLower, "@"); at > 0 {
		local := userLower[:at]
		if local != "" && strings.Contains(passLower, local) {
			return nil, errx.New(codes.InvalidArgument, 422, errx.CodePasswordTooWeak, "PASSWORD_CONTAINS_USERNAME", "password must not contain username", map[string]string{"password": "contains username"})
		}
	}

	// Username 唯一性
	var count int64
	if err := l.svcCtx.AdminUserRepo.GetDB().WithContext(l.ctx).
		Model(&model.AdminUserModel{}).
		Where("username = ?", username).
		Count(&count).Error; err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	if count > 0 {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "USERNAME_EXISTS", "用户名已存在", nil)
	}

	passwordHash, err := security.HashPassword(plainPassword)
	if err != nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "password hash failed", nil)
	}

	current, _ := admininterceptor.GetCurrentAdmin(l.ctx)
	operatorID := int64(0)
	if current != nil {
		operatorID = current.ID
	}

	// 2FA secret：若启用全局 Require2FA 或本账号强制 2FA，则生成 secret
	twoFARequired := in.TwoFactorRequired || l.svcCtx.EffectiveRequire2FA(l.ctx)

	requirePasswordChange := in.RequirePasswordChange
	if tempPassword != "" {
		// 生成临时密码时默认强制改密，避免临时密码长期有效
		requirePasswordChange = true
	}

	newAdmin := &model.AdminUserModel{
		Username:              username,
		PasswordHash:          passwordHash,
		Name:                  strings.TrimSpace(in.Name),
		Role:                  role,
		Status:                "active",
		TwoFactorSecret:       "",
		TwoFactorEnabled:      false,
		RequirePasswordChange: requirePasswordChange,
		TwoFactorRequired:     twoFARequired,
		TokenVersion:          1,
		CreatedBy:             operatorID,
		UpdatedBy:             operatorID,
	}

	ip := middleware.GetClientIP(l.ctx)
	ua := middleware.GetUserAgent(l.ctx)
	detailsBytes, _ := json.Marshal(map[string]interface{}{
		"username":                username,
		"role":                    role,
		"require_password_change": in.RequirePasswordChange,
		"two_factor_required":     in.TwoFactorRequired,
	})

	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		adminRepo := repository.NewAdminUserRepository(tx)
		historyRepo := repository.NewAdminPasswordHistoryRepository(tx)
		auditRepo := repository.NewAdminAuditLogRepository(tx)

		if err := adminRepo.Create(l.ctx, newAdmin); err != nil {
			return err
		}
		// RBAC v2: assign initial role to user
		now := time.Now()
		if err := tx.WithContext(l.ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&model.AdminUserRoleModel{
				UserID:    newAdmin.ID,
				RoleID:    roleModel.ID,
				CreatedAt: &now,
			}).Error; err != nil {
			return errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "assign user role failed", nil)
		}
		if err := historyRepo.Add(l.ctx, newAdmin.ID, passwordHash); err != nil {
			return err
		}
		_ = auditRepo.CreateLog(l.ctx, &model.AdminAuditLogModel{
			AdminID:     operatorID,
			Action:      "admin.create",
			TargetType:  "admin",
			TargetID:    resp.AdminIDString(newAdmin.ID),
			Description: "创建管理员: " + username,
			Details:     detailsBytes,
			IP:          ip,
			UserAgent:   ua,
		})
		return nil
	}); err != nil {
		l.Logger.Errorf("create admin failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	emailSent := false
	if _, err := notify.SendAdminAccountCreatedEmail(username, "Zink Wallet", username, plainPassword, requirePasswordChange, twoFARequired); err != nil {
		l.Logger.Errorf("send admin created email failed: %v", err)
	} else {
		emailSent = true
	}

	return &pb.CreateAdminResponse{
		Success: true,
		Message: resp.Msg(l.ctx, "ADMIN_CREATED"),
		Data: &pb.CreateAdminData{
			AdminId:             resp.AdminIDString(newAdmin.ID),
			Username:            newAdmin.Username,
			Name:                newAdmin.Name,
			Role:                newAdmin.Role,
			Status:              newAdmin.Status,
			CreatedAt:           formatTime(newAdmin.CreatedAt),
			TempPassword:        tempPassword,
			EmailSent:           emailSent,
			TwoFactorSecret:     "",
			TwoFactorOtpauthUrl: "",
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}
