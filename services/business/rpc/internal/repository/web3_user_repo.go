package repository

import (
	"context"
	"errors"
	"fmt"
	"internalwallet/services/business/rpc/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Web3UserListFilter struct {
	AddressStatus    string // normal | blacklisted
	TwoFactorEnabled *bool
	BiometricEnabled *bool
	Network          string
	Keyword          string
	CreatedFrom      *time.Time
	CreatedTo        *time.Time
	SortBy           string // created_at | last_active_at
	SortOrder        string // asc | desc
}

type Web3UserSummaryCounts struct {
	TotalDevices         int64
	TotalAddresses       int64
	BlacklistedAddresses int64
	TwoFactorEnabled     int64
}

type Web3UserRepository interface {
	WithTx(tx *gorm.DB) Web3UserRepository
	GetDB() *gorm.DB

	FindByDeviceID(ctx context.Context, deviceID string) (*model.Web3UserModel, error)
	List(ctx context.Context, page, pageSize int32, f Web3UserListFilter) ([]*model.Web3UserModel, int64, error)
	Summary(ctx context.Context, f Web3UserListFilter) (Web3UserSummaryCounts, error)

	// Trade password methods
	SetTradePassword(ctx context.Context, deviceID, passwordHash string) error
	GetTradePasswordHash(ctx context.Context, deviceID string) (hash string, hasPassword bool, err error)

	// Update last active time
	UpdateLastActiveAt(ctx context.Context, deviceID string) error
}

type web3UserRepo struct {
	db *gorm.DB
}

func NewWeb3UserRepository(db *gorm.DB) Web3UserRepository    { return &web3UserRepo{db: db} }
func (r *web3UserRepo) WithTx(tx *gorm.DB) Web3UserRepository { return &web3UserRepo{db: tx} }
func (r *web3UserRepo) GetDB() *gorm.DB                       { return r.db }

func applyWeb3UserFilters(q *gorm.DB, f Web3UserListFilter) *gorm.DB {
	q = q.Where("deleted_at IS NULL")

	if f.CreatedFrom != nil && !f.CreatedFrom.IsZero() {
		q = q.Where("created_at >= ?", *f.CreatedFrom)
	}
	if f.CreatedTo != nil && !f.CreatedTo.IsZero() {
		q = q.Where("created_at <= ?", *f.CreatedTo)
	}
	if f.TwoFactorEnabled != nil {
		q = q.Where("two_factor_enabled = ?", *f.TwoFactorEnabled)
	}
	if f.BiometricEnabled != nil {
		q = q.Where("biometric_enabled = ?", *f.BiometricEnabled)
	}

	network := strings.TrimSpace(f.Network)
	if network != "" {
		q = q.Where(
			"EXISTS (SELECT 1 FROM web3_user_addresses a WHERE a.deleted_at IS NULL AND a.web3_user_id = web3_users.id AND a.network = ?)",
			network,
		)
	}

	switch strings.TrimSpace(f.AddressStatus) {
	case "":
		// no-op
	case "blacklisted":
		q = q.Where(
			"EXISTS (SELECT 1 FROM web3_user_addresses a WHERE a.deleted_at IS NULL AND a.web3_user_id = web3_users.id AND a.is_blacklisted = 1)",
		)
	case "normal":
		q = q.Where(
			"NOT EXISTS (SELECT 1 FROM web3_user_addresses a WHERE a.deleted_at IS NULL AND a.web3_user_id = web3_users.id AND a.is_blacklisted = 1)",
		)
	default:
		// keep query unchanged; validation should happen in logic
	}

	kw := strings.TrimSpace(f.Keyword)
	if kw != "" {
		like := "%" + kw + "%"
		q = q.Where(
			"(device_id LIKE ? OR EXISTS (SELECT 1 FROM web3_user_addresses a WHERE a.deleted_at IS NULL AND a.web3_user_id = web3_users.id AND a.address LIKE ?))",
			like,
			like,
		)
	}

	return q
}

func (r *web3UserRepo) FindByDeviceID(ctx context.Context, deviceID string) (*model.Web3UserModel, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, fmt.Errorf("web3 user not found")
	}
	var m model.Web3UserModel
	err := r.db.WithContext(ctx).
		Model(&model.Web3UserModel{}).
		Where("device_id = ? AND deleted_at IS NULL", deviceID).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("web3 user not found")
	}
	return &m, err
}

func (r *web3UserRepo) List(ctx context.Context, page, pageSize int32, f Web3UserListFilter) ([]*model.Web3UserModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := applyWeb3UserFilters(r.db.WithContext(ctx).Model(&model.Web3UserModel{}), f)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderCol := "created_at"
	switch strings.TrimSpace(f.SortBy) {
	case "", "created_at":
		orderCol = "created_at"
	case "last_active_at":
		orderCol = "last_active_at"
	}
	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(f.SortOrder), "asc") {
		orderDir = "ASC"
	}

	var items []*model.Web3UserModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).Offset(offset).Limit(int(pageSize)).Find(&items).Error
	return items, total, err
}

func (r *web3UserRepo) Summary(ctx context.Context, f Web3UserListFilter) (Web3UserSummaryCounts, error) {
	var out Web3UserSummaryCounts

	// total devices
	qDevices := applyWeb3UserFilters(r.db.WithContext(ctx).Model(&model.Web3UserModel{}), f)
	if err := qDevices.Count(&out.TotalDevices).Error; err != nil {
		return out, err
	}

	// 2FA enabled count (within filtered dataset)
	q2fa := applyWeb3UserFilters(r.db.WithContext(ctx).Model(&model.Web3UserModel{}), f).Where("two_factor_enabled = 1")
	if err := q2fa.Count(&out.TwoFactorEnabled).Error; err != nil {
		return out, err
	}

	// addresses counts for filtered devices
	idSub := applyWeb3UserFilters(r.db.WithContext(ctx).Model(&model.Web3UserModel{}).Select("id"), f)
	qAddr := r.db.WithContext(ctx).
		Table("web3_user_addresses a").
		Joins("JOIN (?) u ON u.id = a.web3_user_id", idSub).
		Where("a.deleted_at IS NULL")
	if err := qAddr.Count(&out.TotalAddresses).Error; err != nil {
		return out, err
	}
	qBlack := r.db.WithContext(ctx).
		Table("web3_user_addresses a").
		Joins("JOIN (?) u ON u.id = a.web3_user_id", idSub).
		Where("a.deleted_at IS NULL AND a.is_blacklisted = 1")
	if err := qBlack.Count(&out.BlacklistedAddresses).Error; err != nil {
		return out, err
	}

	return out, nil
}

// SetTradePassword sets the trade password hash for a Web3 user
func (r *web3UserRepo) SetTradePassword(ctx context.Context, deviceID, passwordHash string) error {
	return r.db.WithContext(ctx).
		Model(&model.Web3UserModel{}).
		Where("device_id = ? AND deleted_at IS NULL", deviceID).
		Updates(map[string]interface{}{
			"trade_password_hash": passwordHash,
			"has_trade_password":  true,
		}).Error
}

// GetTradePasswordHash retrieves trade password hash and status for a Web3 user
func (r *web3UserRepo) GetTradePasswordHash(ctx context.Context, deviceID string) (hash string, hasPassword bool, err error) {
	var user model.Web3UserModel
	err = r.db.WithContext(ctx).
		Select("trade_password_hash", "has_trade_password").
		Where("device_id = ? AND deleted_at IS NULL", deviceID).
		First(&user).Error
	if err != nil {
		return "", false, err
	}
	return user.TradePasswordHash, user.HasTradePassword, nil
}

// UpdateLastActiveAt updates the last active time for a Web3 user
func (r *web3UserRepo) UpdateLastActiveAt(ctx context.Context, deviceID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Web3UserModel{}).
		Where("device_id = ? AND deleted_at IS NULL", deviceID).
		Update("last_active_at", now).Error
}

type Web3UserAddressAgg struct {
	Web3UserID     int64  `gorm:"column:web3_user_id"`
	WalletCount    int64  `gorm:"column:wallet_count"`
	Networks       string `gorm:"column:networks"`
	PrimaryAddress string `gorm:"column:primary_address"`
	HasBlacklisted int64  `gorm:"column:has_blacklisted"`
}

type Web3UserAddressRepository interface {
	WithTx(tx *gorm.DB) Web3UserAddressRepository
	GetDB() *gorm.DB

	Create(ctx context.Context, addr *model.Web3UserAddressModel) error
	ListByUserID(ctx context.Context, userID int64) ([]*model.Web3UserAddressModel, error)
	FindByUserIDAndChainIDAndAddress(ctx context.Context, userID int64, chainID int64, address string) (*model.Web3UserAddressModel, error)
	FindByAddress(ctx context.Context, address string) (*model.Web3UserAddressModel, error)
	AggregateByUserIDs(ctx context.Context, userIDs []int64) (map[int64]Web3UserAddressAgg, error)
}

type web3UserAddressRepo struct {
	db *gorm.DB
}

func NewWeb3UserAddressRepository(db *gorm.DB) Web3UserAddressRepository {
	return &web3UserAddressRepo{db: db}
}
func (r *web3UserAddressRepo) WithTx(tx *gorm.DB) Web3UserAddressRepository {
	return &web3UserAddressRepo{db: tx}
}
func (r *web3UserAddressRepo) GetDB() *gorm.DB { return r.db }

func (r *web3UserAddressRepo) Create(ctx context.Context, addr *model.Web3UserAddressModel) error {
	if addr == nil {
		return fmt.Errorf("address model is nil")
	}
	return r.db.WithContext(ctx).Create(addr).Error
}

func (r *web3UserAddressRepo) ListByUserID(ctx context.Context, userID int64) ([]*model.Web3UserAddressModel, error) {
	if userID <= 0 {
		return nil, nil
	}
	var items []*model.Web3UserAddressModel
	err := r.db.WithContext(ctx).
		Model(&model.Web3UserAddressModel{}).
		Where("web3_user_id = ? AND deleted_at IS NULL", userID).
		Order("is_primary DESC, id ASC").
		Find(&items).Error
	return items, err
}

func (r *web3UserAddressRepo) FindByUserIDAndChainIDAndAddress(ctx context.Context, userID int64, chainID int64, address string) (*model.Web3UserAddressModel, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("address not found")
	}
	if chainID <= 0 {
		return nil, fmt.Errorf("address not found")
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, fmt.Errorf("address not found")
	}
	var m model.Web3UserAddressModel
	q := r.db.WithContext(ctx).
		Model(&model.Web3UserAddressModel{}).
		Where("web3_user_id = ? AND chain_id = ? AND deleted_at IS NULL", userID, chainID)
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		q = q.Where("LOWER(address) = ?", strings.ToLower(address))
	} else {
		q = q.Where("address = ?", address)
	}
	err := q.First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("address not found")
	}
	return &m, err
}

// FindByAddress 根据地址查询（公开查询，不限制用户）
func (r *web3UserAddressRepo) FindByAddress(ctx context.Context, address string) (*model.Web3UserAddressModel, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, fmt.Errorf("address not found")
	}
	var m model.Web3UserAddressModel
	q := r.db.WithContext(ctx).
		Model(&model.Web3UserAddressModel{}).
		Where("deleted_at IS NULL")
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		q = q.Where("LOWER(address) = ?", strings.ToLower(address))
	} else {
		q = q.Where("address = ?", address)
	}
	err := q.First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("address not found")
	}
	return &m, err
}

func (r *web3UserAddressRepo) AggregateByUserIDs(ctx context.Context, userIDs []int64) (map[int64]Web3UserAddressAgg, error) {
	out := make(map[int64]Web3UserAddressAgg, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	var rows []Web3UserAddressAgg
	err := r.db.WithContext(ctx).
		Table("web3_user_addresses").
		Select(
			"web3_user_id, COUNT(*) AS wallet_count, "+
				"GROUP_CONCAT(DISTINCT network ORDER BY network SEPARATOR ',') AS networks, "+
				"MAX(CASE WHEN is_primary = 1 THEN address ELSE '' END) AS primary_address, "+
				"MAX(CASE WHEN is_blacklisted = 1 THEN 1 ELSE 0 END) AS has_blacklisted",
		).
		Where("deleted_at IS NULL AND web3_user_id IN ?", userIDs).
		Group("web3_user_id").
		Find(&rows).Error
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		out[r.Web3UserID] = r
	}
	return out, nil
}
