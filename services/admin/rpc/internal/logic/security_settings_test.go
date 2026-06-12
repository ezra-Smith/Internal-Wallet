package logic

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
	"time"

	commonrepo "internalwallet/common/repository"
	"internalwallet/proto/pb"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/security"
	"internalwallet/services/admin/rpc/internal/svc"

	"gorm.io/gorm"
)

type fakeAdminUserRepo struct {
	admin *model.AdminUserModel
}

// ---- BaseRepository stubs ----

func (r *fakeAdminUserRepo) Create(context.Context, *model.AdminUserModel) error { return nil }
func (r *fakeAdminUserRepo) CreateMultiple(context.Context, []*model.AdminUserModel) error {
	return nil
}
func (r *fakeAdminUserRepo) Update(context.Context, *model.AdminUserModel) error { return nil }
func (r *fakeAdminUserRepo) UpdateFields(context.Context, int64, map[string]interface{}) error {
	return nil
}
func (r *fakeAdminUserRepo) Delete(context.Context, int64) error     { return nil }
func (r *fakeAdminUserRepo) SoftDelete(context.Context, int64) error { return nil }
func (r *fakeAdminUserRepo) FindAll(context.Context) ([]*model.AdminUserModel, error) {
	return nil, nil
}
func (r *fakeAdminUserRepo) FindByCondition(context.Context, map[string]interface{}) ([]*model.AdminUserModel, error) {
	return nil, nil
}
func (r *fakeAdminUserRepo) Count(context.Context, map[string]interface{}) (int64, error) {
	return 0, nil
}
func (r *fakeAdminUserRepo) Exists(context.Context, int64) (bool, error) { return false, nil }
func (r *fakeAdminUserRepo) FindWithPagination(context.Context, int, int, map[string]interface{}) ([]*model.AdminUserModel, int64, error) {
	return nil, 0, nil
}
func (r *fakeAdminUserRepo) WithTx(*gorm.DB) commonrepo.BaseRepository[model.AdminUserModel] {
	return r
}
func (r *fakeAdminUserRepo) GetDB() *gorm.DB { return nil }

func (r *fakeAdminUserRepo) FindByID(_ context.Context, id int64) (*model.AdminUserModel, error) {
	if r.admin == nil || r.admin.ID != id {
		return nil, fmt.Errorf("admin not found")
	}
	return r.admin, nil
}

// ---- AdminUserRepository stubs ----

func (r *fakeAdminUserRepo) GetByUsername(context.Context, string) (*model.AdminUserModel, error) {
	return nil, fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) List(context.Context, int32, int32, string, string, string) ([]*model.AdminUserModel, int64, error) {
	return nil, 0, fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) CountActiveSuperAdmins(context.Context) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) IncrementFailedLogin(context.Context, int64, *time.Time) (int, error) {
	return 0, fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) ResetFailedLogin(context.Context, int64) error {
	return fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) UpdateLoginSuccess(context.Context, int64, string) error { return nil }
func (r *fakeAdminUserRepo) SetLockUntil(context.Context, int64, *time.Time, int64) error {
	return fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) UpdatePassword(context.Context, int64, string, bool, int64) error {
	return fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) UpdateRole(context.Context, int64, string, int64) error {
	return fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) UpdateStatus(context.Context, int64, string, int64) error {
	return fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) UpdateTwoFactorSecret(context.Context, int64, string, int64) error {
	return fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) UpdateTwoFactorPendingSecret(_ context.Context, id int64, secret string, _ int64) error {
	if r.admin == nil || r.admin.ID != id {
		return fmt.Errorf("admin not found")
	}
	r.admin.TwoFactorPendingSecret = secret
	return nil
}
func (r *fakeAdminUserRepo) SetTwoFactorEnabled(context.Context, int64, bool, int64) error {
	return fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) ResetTwoFactor(context.Context, int64, int64) error {
	return fmt.Errorf("not implemented")
}
func (r *fakeAdminUserRepo) PromoteTwoFactorPending(_ context.Context, id int64, activeSecret string, boundAt *time.Time, _ int64) error {
	if r.admin == nil || r.admin.ID != id {
		return fmt.Errorf("admin not found")
	}
	r.admin.TwoFactorSecret = activeSecret
	r.admin.TwoFactorPendingSecret = ""
	r.admin.TwoFactorEnabled = true
	r.admin.TwoFactorBoundAt = boundAt
	return nil
}
func (r *fakeAdminUserRepo) DisableTwoFactor(_ context.Context, id int64, _ int64) error {
	if r.admin == nil || r.admin.ID != id {
		return fmt.Errorf("admin not found")
	}
	r.admin.TwoFactorSecret = ""
	r.admin.TwoFactorPendingSecret = ""
	r.admin.TwoFactorEnabled = false
	r.admin.TwoFactorBoundAt = nil
	return nil
}
func (r *fakeAdminUserRepo) IncrementTokenVersion(context.Context, int64, int64) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

func totpCode(secret string, at time.Time) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	key, err := enc.DecodeString(normalized)
	if err != nil {
		key, err = base32.StdEncoding.DecodeString(normalized)
		if err != nil {
			return "", fmt.Errorf("invalid totp secret")
		}
	}

	counter := uint64(at.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binCode := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)

	otp := binCode % 1000000
	return fmt.Sprintf("%06d", otp), nil
}

func TestConfirmTwoFASetup_Success(t *testing.T) {
	t.Setenv("ADMIN_2FA_SECRET_ENC_KEY", "")

	const (
		adminID     int64 = 10001
		username          = "super_admin@company.com"
		password          = "ChangeMe123!"
		keyMaterial       = "k1"
	)

	hash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}

	secretPlain := "JBSWY3DPEHPK3PXP"
	secretEnc, err := security.EncryptSecretForStorage(secretPlain, keyMaterial)
	if err != nil {
		t.Fatalf("EncryptSecretForStorage error: %v", err)
	}

	admin := &model.AdminUserModel{
		Username:               username,
		PasswordHash:           hash,
		TwoFactorEnabled:       false,
		TwoFactorSecret:        "",
		TwoFactorPendingSecret: secretEnc,
	}
	admin.ID = adminID

	repo := &fakeAdminUserRepo{admin: admin}
	svcCtx := &svc.ServiceContext{}
	svcCtx.Config.JWT.RefreshSecret = keyMaterial
	svcCtx.AdminUserRepo = repo

	currentAdmin := &model.AdminUserModel{}
	currentAdmin.ID = adminID
	ctx := admininterceptor.ContextWithCurrentAdmin(context.Background(), currentAdmin)

	code, err := totpCode(secretPlain, time.Now())
	if err != nil {
		t.Fatalf("totpCode error: %v", err)
	}

	l := NewConfirmTwoFASetupLogic(ctx, svcCtx)
	resp, err := l.ConfirmTwoFASetup(&pb.ConfirmTwoFASetupRequest{
		CurrentPassword: password,
		TotpCode:        code,
	})
	if err != nil {
		t.Fatalf("ConfirmTwoFASetup error: %v", err)
	}
	if resp == nil || !resp.Success || resp.Data == nil || !resp.Data.Enabled {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if !admin.TwoFactorEnabled || admin.TwoFactorPendingSecret != "" || admin.TwoFactorSecret == "" || admin.TwoFactorBoundAt == nil {
		t.Fatalf("admin not updated as expected: %#v", admin)
	}
}

func TestDisableTwoFA_Success(t *testing.T) {
	t.Setenv("ADMIN_2FA_SECRET_ENC_KEY", "")

	const (
		adminID           int64 = 10001
		username                = "super_admin@company.com"
		password                = "ChangeMe123!"
		keyMaterial             = "k1"
		activeSecretPlain       = "JBSWY3DPEHPK3PXP"
	)

	hash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}

	activeEnc, err := security.EncryptSecretForStorage(activeSecretPlain, keyMaterial)
	if err != nil {
		t.Fatalf("EncryptSecretForStorage error: %v", err)
	}

	now := time.Now()
	admin := &model.AdminUserModel{
		Username:         username,
		PasswordHash:     hash,
		TwoFactorEnabled: true,
		TwoFactorSecret:  activeEnc,
		TwoFactorBoundAt: &now,
	}
	admin.ID = adminID

	repo := &fakeAdminUserRepo{admin: admin}
	svcCtx := &svc.ServiceContext{}
	svcCtx.Config.JWT.RefreshSecret = keyMaterial
	svcCtx.Config.Security.Require2FA = false
	svcCtx.AdminUserRepo = repo

	currentAdmin := &model.AdminUserModel{}
	currentAdmin.ID = adminID
	ctx := admininterceptor.ContextWithCurrentAdmin(context.Background(), currentAdmin)

	code, err := totpCode(activeSecretPlain, time.Now())
	if err != nil {
		t.Fatalf("totpCode error: %v", err)
	}

	l := NewDisableTwoFALogic(ctx, svcCtx)
	resp, err := l.DisableTwoFA(&pb.DisableTwoFARequest{
		CurrentPassword: password,
		TotpCode:        code,
	})
	if err != nil {
		t.Fatalf("DisableTwoFA error: %v", err)
	}
	if resp == nil || !resp.Success || resp.Data == nil || resp.Data.Enabled {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if admin.TwoFactorEnabled || admin.TwoFactorSecret != "" || admin.TwoFactorBoundAt != nil {
		t.Fatalf("admin not disabled as expected: %#v", admin)
	}
}

func TestStartTwoFASetup_UsesPendingSecret(t *testing.T) {
	t.Setenv("ADMIN_2FA_SECRET_ENC_KEY", "")

	const (
		adminID            int64 = 10001
		username                 = "super_admin@company.com"
		keyMaterial              = "k1"
		pendingSecretPlain       = "JBSWY3DPEHPK3PXP"
	)

	pendingEnc, err := security.EncryptSecretForStorage(pendingSecretPlain, keyMaterial)
	if err != nil {
		t.Fatalf("EncryptSecretForStorage error: %v", err)
	}

	admin := &model.AdminUserModel{
		Username:               username,
		TwoFactorEnabled:       false,
		TwoFactorSecret:        "",
		TwoFactorPendingSecret: pendingEnc,
	}
	admin.ID = adminID

	repo := &fakeAdminUserRepo{admin: admin}
	svcCtx := &svc.ServiceContext{}
	svcCtx.Config.JWT.RefreshSecret = keyMaterial
	svcCtx.AdminUserRepo = repo

	currentAdmin := &model.AdminUserModel{}
	currentAdmin.ID = adminID
	ctx := admininterceptor.ContextWithCurrentAdmin(context.Background(), currentAdmin)

	l := NewStartTwoFASetupLogic(ctx, svcCtx)
	resp, err := l.StartTwoFASetup(&pb.StartTwoFASetupRequest{})
	if err != nil {
		t.Fatalf("StartTwoFASetup error: %v", err)
	}
	if resp == nil || !resp.Success || resp.Data == nil {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.Data.Secret != pendingSecretPlain {
		t.Fatalf("expected secret %q, got %q", pendingSecretPlain, resp.Data.Secret)
	}
	if !strings.HasPrefix(resp.Data.QrCode, "data:image/png;base64,") {
		t.Fatalf("expected qr_code data URL, got %q", resp.Data.QrCode)
	}
	if strings.TrimSpace(resp.Data.OtpauthUrl) == "" {
		t.Fatalf("expected otpauth_url to be set")
	}
}
