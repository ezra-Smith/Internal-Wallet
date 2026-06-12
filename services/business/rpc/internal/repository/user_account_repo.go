package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

// ErrProfileNotFound is returned when a user profile does not exist.
// Use errors.Is(err, ErrProfileNotFound) to check.
var ErrProfileNotFound = errors.New("profile not found")

// UserAccountRepository 用户账户 Repository 接口
type UserAccountRepository interface {
	commonRepo.BaseRepository[model.UserModel]

	FindByUserIDAndType(ctx context.Context, userID int64, accountType int) (*model.UserModel, error)
	FindByUserID(ctx context.Context, userID int64) ([]*model.UserModel, error)
	ExistsByUserIDAndType(ctx context.Context, userID int64, accountType int) (bool, error)
	UpdateStatus(ctx context.Context, id int64, status int) error
	FindByAccountType(ctx context.Context, accountType int, page, pageSize int) ([]*model.UserModel, int64, error)
	GetByID(ctx context.Context, id int64) (*model.UserModel, error)
	GetByUID(ctx context.Context, uid string) (*model.UserModel, error)
	GetByPhone(ctx context.Context, phone, countryCode string) (*model.UserModel, error)
	GetByPhoneOnly(ctx context.Context, phone string) (*model.UserModel, error)
	GetByEmail(ctx context.Context, email string) (*model.UserModel, error)
	CreateMember(ctx context.Context, m *model.UserModel) error
	UpdatePreferences(ctx context.Context, id int64, language, priceUnit int) error
	SetHasTradePassword(ctx context.Context, id int64, has bool) error
	SetTradePassword(ctx context.Context, id int64, passwordHash string) error
	GetTradePasswordHash(ctx context.Context, id int64) (string, bool, error)
	GetAuthByID(ctx context.Context, id int64) (*model.UserModel, error)
	UpsertEmailByID(ctx context.Context, id int64, email string) error
	UpsertPhoneByID(ctx context.Context, id int64, phone string, countryCode string) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	UpsertGoogleAuth(ctx context.Context, id int64, secret string, backupCodes string, enabled bool) error
	UpdateLastLoginTime(ctx context.Context, id int64, ip string) error
}

type userAccountRepo struct {
	commonRepo.BaseRepository[model.UserModel]
}

// NewUserAccountRepository 创建用户账户 Repository
func NewUserAccountRepository(db *gorm.DB) UserAccountRepository {
	return &userAccountRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserModel](db),
	}
}

// ==================== 以下是业务特定方法（通用方法已通过组合自动拥有） ====================

// FindByUserIDAndType 根据用户ID和账户类型查询
func (r *userAccountRepo) FindByUserIDAndType(ctx context.Context, userID int64, accountType int) (*model.UserModel, error) {
	var profile model.UserModel
	err := r.GetDB().WithContext(context.Background()).
		Where("id = ?", userID).
		First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProfileNotFound
	}
	return &profile, err
}

// FindByUserID 查询用户的所有账户
func (r *userAccountRepo) FindByUserID(ctx context.Context, userID int64) ([]*model.UserModel, error) {
	var profiles []*model.UserModel
	err := r.GetDB().WithContext(context.Background()).
		Where("id = ?", userID).
		Order("created_at DESC").
		Find(&profiles).Error
	return profiles, err
}

// ExistsByUserIDAndType 检查账户是否存在
func (r *userAccountRepo) ExistsByUserIDAndType(ctx context.Context, userID int64, accountType int) (bool, error) {
	var count int64
	err := r.GetDB().WithContext(context.Background()).
		Model(&model.UserModel{}).
		Where("id = ?", userID).
		Count(&count).Error
	return count > 0, err
}

// UpdateStatus 更新账户状态
func (r *userAccountRepo) UpdateStatus(ctx context.Context, id int64, status int) error {
	return r.UpdateFields(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// FindByAccountType 根据账户类型分页查询
func (r *userAccountRepo) FindByAccountType(ctx context.Context, accountType int, page, pageSize int) ([]*model.UserModel, int64, error) {
	var profiles []*model.UserModel
	var total int64
	db := r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("member_level = ?", accountType)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	err := db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&profiles).Error
	return profiles, total, err
}

func (r *userAccountRepo) GetByID(ctx context.Context, id int64) (*model.UserModel, error) {
	var m model.UserModel
	err := r.GetDB().WithContext(context.Background()).Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProfileNotFound
	}
	return &m, err
}

func (r *userAccountRepo) GetByUID(ctx context.Context, uid string) (*model.UserModel, error) {
	var m model.UserModel
	err := r.GetDB().WithContext(context.Background()).Where("id = ?", uid).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProfileNotFound
	}
	return &m, err
}

func (r *userAccountRepo) GetByPhone(ctx context.Context, phone, countryCode string) (*model.UserModel, error) {
	var m model.UserModel
	// Backward compatible query:
	// - historical rows may store country_code without '+' (e.g. "86" / "1")
	// - new logic normalizes to "+86"
	legacy := strings.TrimPrefix(countryCode, "+")
	err := r.GetDB().WithContext(context.Background()).
		Where("phone = ? AND (country_code = ? OR country_code = ?)", phone, countryCode, legacy).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProfileNotFound
	}
	return &m, err
}

func (r *userAccountRepo) GetByPhoneOnly(ctx context.Context, phone string) (*model.UserModel, error) {
	var m model.UserModel
	err := r.GetDB().WithContext(context.Background()).Where("phone = ?", phone).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProfileNotFound
	}
	return &m, err
}

func (r *userAccountRepo) GetByEmail(ctx context.Context, email string) (*model.UserModel, error) {
	var m model.UserModel
	err := r.GetDB().WithContext(context.Background()).Where("email = ?", email).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProfileNotFound
	}
	return &m, err
}

func (r *userAccountRepo) CreateMember(ctx context.Context, m *model.UserModel) error {
	return r.GetDB().WithContext(context.Background()).Create(m).Error
}

func (r *userAccountRepo) UpdatePreferences(ctx context.Context, id int64, language, priceUnit int) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"language":   language,
		"price_unit": priceUnit,
	}).Error
}

func (r *userAccountRepo) SetHasTradePassword(ctx context.Context, id int64, has bool) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"has_trade_password": has,
	}).Error
}

func (r *userAccountRepo) SetTradePassword(ctx context.Context, id int64, passwordHash string) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"trade_password_hash": passwordHash,
		"has_trade_password":  true,
	}).Error
}

func (r *userAccountRepo) GetTradePasswordHash(ctx context.Context, id int64) (string, bool, error) {
	var m model.UserModel
	err := r.GetDB().WithContext(context.Background()).
		Select("trade_password_hash, has_trade_password").
		Where("id = ?", id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, ErrProfileNotFound
	}
	if err != nil {
		return "", false, err
	}
	return m.TradePasswordHash, m.HasTradePassword, nil
}

func (r *userAccountRepo) GetAuthByID(ctx context.Context, id int64) (*model.UserModel, error) {
	var u model.UserModel
	err := r.GetDB().WithContext(context.Background()).
		Select("id, password_hash, email, phone, country_code, google_auth_secret, is_2fa_enabled").
		Where("id = ?", id).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("profile not found")
	}
	return &u, err
}

func (r *userAccountRepo) UpsertEmailByID(ctx context.Context, id int64, email string) error {
	var count int64
	if err := r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		m := &model.UserModel{Email: email}
		m.ID = id
		return r.GetDB().WithContext(context.Background()).Create(m).Error
	}
	return r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Updates(map[string]interface{}{"email": email}).Error
}

func (r *userAccountRepo) UpsertPhoneByID(ctx context.Context, id int64, phone string, countryCode string) error {
	var count int64
	if err := r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		m := &model.UserModel{Phone: phone, CountryCode: countryCode}
		m.ID = id
		return r.GetDB().WithContext(context.Background()).Create(m).Error
	}
	return r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Updates(map[string]interface{}{"phone": phone, "country_code": countryCode}).Error
}

// UpdatePassword 更新密码哈希（使用 bcrypt）
func (r *userAccountRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"password_hash": passwordHash,
	}).Error
}

func (r *userAccountRepo) UpsertGoogleAuth(ctx context.Context, id int64, secret string, backupCodes string, enabled bool) error {
	var count int64
	if err := r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		m := &model.UserModel{GoogleAuthSecret: secret, BackupCodes: backupCodes, Is2faEnabled: enabled}
		m.ID = id
		return r.GetDB().WithContext(context.Background()).Create(m).Error
	}
	return r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"google_auth_secret": secret,
		"backup_codes":       backupCodes,
		"is_2fa_enabled":     enabled,
	}).Error
}

// UpdateLastLoginTime 更新用户最后登录时间和IP
func (r *userAccountRepo) UpdateLastLoginTime(ctx context.Context, id int64, ip string) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_login_time": time.Now(),
		"last_login_ip":   ip,
	}).Error
}
