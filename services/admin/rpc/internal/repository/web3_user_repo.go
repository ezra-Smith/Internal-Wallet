package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"internalwallet/services/admin/rpc/internal/model"

	"gorm.io/gorm"
)

type Web3UserListFilter struct {
	// Device filters (web3_users table)
	DeviceID         string // 设备ID精确搜索
	Platform         string // iOS | Android
	OsVersion        string // 系统版本
	AppVersion       string // 应用版本
	DeviceModel      string // 设备型号
	DeviceName       string // 设备名称
	Locale           string // 语言/地区
	TwoFactorEnabled *bool  // 2FA启用
	BiometricEnabled *bool  // 生物识别启用
	HasTradePassword *bool  // 是否有交易密码

	// Address filters (web3_user_addresses table)
	Network        string // 网络筛选
	AddressStatus  string // normal | blacklisted
	PrimaryAddress string // 主地址搜索（模糊匹配）

	// Wallet count filter (calculated)
	WalletCountMin *int32 // 钱包数量最小值
	WalletCountMax *int32 // 钱包数量最大值

	// Time range
	CreatedFrom    *time.Time // 创建开始时间
	CreatedTo      *time.Time // 创建结束时间
	LastActiveFrom *time.Time // 最后活跃开始时间
	LastActiveTo   *time.Time // 最后活跃结束时间

	// General search
	Keyword string // 关键词搜索（device_id或address）

	// Sort
	SortBy    string // created_at | last_active_at
	SortOrder string // asc | desc
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

	// New methods for GetWeb3UsersPageList
	ListWithFilter(ctx context.Context, page, pageSize int32, f Web3UserListFilter) ([]*model.Web3UserModel, int64, error)

	// New methods for GetWeb3UsersSummary
	CountAll(ctx context.Context) (int64, error)
	Count2FAEnabled(ctx context.Context) (int64, error)
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

// ListWithFilter queries web3_users with comprehensive filtering
func (r *web3UserRepo) ListWithFilter(ctx context.Context, page, pageSize int32, f Web3UserListFilter) ([]*model.Web3UserModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&model.Web3UserModel{}).Where("deleted_at IS NULL")

	// Apply all filters
	query = applyWeb3UserFiltersExtended(query, f)

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	orderCol := "created_at"
	if strings.TrimSpace(f.SortBy) == "last_active_at" {
		orderCol = "last_active_at"
	}
	orderDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(f.SortOrder), "asc") {
		orderDir = "ASC"
	}

	// Query
	var items []*model.Web3UserModel
	offset := int((page - 1) * pageSize)
	err := query.Order(orderCol + " " + orderDir).
		Offset(offset).Limit(int(pageSize)).
		Find(&items).Error

	return items, total, err
}

// CountAll returns total count of all web3 users
func (r *web3UserRepo) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Web3UserModel{}).
		Where("deleted_at IS NULL").
		Count(&count).Error
	return count, err
}

// Count2FAEnabled returns count of users with 2FA enabled
func (r *web3UserRepo) Count2FAEnabled(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Web3UserModel{}).
		Where("deleted_at IS NULL AND two_factor_enabled = 1").
		Count(&count).Error
	return count, err
}

// applyWeb3UserFiltersExtended applies comprehensive filters for ListWithFilter
func applyWeb3UserFiltersExtended(q *gorm.DB, f Web3UserListFilter) *gorm.DB {
	// Device ID (exact match)
	if v := strings.TrimSpace(f.DeviceID); v != "" {
		q = q.Where("device_id = ?", v)
	}

	// Platform
	if v := strings.TrimSpace(f.Platform); v != "" {
		q = q.Where("platform = ?", v)
	}

	// OS Version
	if v := strings.TrimSpace(f.OsVersion); v != "" {
		q = q.Where("os_version = ?", v)
	}

	// App Version
	if v := strings.TrimSpace(f.AppVersion); v != "" {
		q = q.Where("app_version = ?", v)
	}

	// Device Model
	if v := strings.TrimSpace(f.DeviceModel); v != "" {
		q = q.Where("device_model LIKE ?", "%"+v+"%")
	}

	// Device Name
	if v := strings.TrimSpace(f.DeviceName); v != "" {
		q = q.Where("device_name LIKE ?", "%"+v+"%")
	}

	// Locale
	if v := strings.TrimSpace(f.Locale); v != "" {
		q = q.Where("locale = ?", v)
	}

	// 2FA Enabled
	if f.TwoFactorEnabled != nil {
		q = q.Where("two_factor_enabled = ?", *f.TwoFactorEnabled)
	}

	// Biometric Enabled
	if f.BiometricEnabled != nil {
		q = q.Where("biometric_enabled = ?", *f.BiometricEnabled)
	}

	// Has Trade Password
	if f.HasTradePassword != nil {
		q = q.Where("has_trade_password = ?", *f.HasTradePassword)
	}

	// Time range filters
	if f.CreatedFrom != nil && !f.CreatedFrom.IsZero() {
		q = q.Where("created_at >= ?", f.CreatedFrom)
	}
	if f.CreatedTo != nil && !f.CreatedTo.IsZero() {
		q = q.Where("created_at <= ?", f.CreatedTo)
	}
	if f.LastActiveFrom != nil && !f.LastActiveFrom.IsZero() {
		q = q.Where("last_active_at >= ?", f.LastActiveFrom)
	}
	if f.LastActiveTo != nil && !f.LastActiveTo.IsZero() {
		q = q.Where("last_active_at <= ?", f.LastActiveTo)
	}

	// Address-related filters (require EXISTS subquery)
	if v := strings.TrimSpace(f.Network); v != "" {
		q = q.Where("EXISTS (SELECT 1 FROM web3_user_addresses WHERE web3_user_addresses.web3_user_id = web3_users.id AND web3_user_addresses.network = ? AND web3_user_addresses.deleted_at IS NULL)", v)
	}

	if v := strings.TrimSpace(f.AddressStatus); v != "" {
		if v == "blacklisted" {
			q = q.Where("EXISTS (SELECT 1 FROM web3_user_addresses WHERE web3_user_addresses.web3_user_id = web3_users.id AND web3_user_addresses.is_blacklisted = 1 AND web3_user_addresses.deleted_at IS NULL)")
		} else if v == "normal" {
			q = q.Where("NOT EXISTS (SELECT 1 FROM web3_user_addresses WHERE web3_user_addresses.web3_user_id = web3_users.id AND web3_user_addresses.is_blacklisted = 1 AND web3_user_addresses.deleted_at IS NULL)")
		}
	}

	if v := strings.TrimSpace(f.PrimaryAddress); v != "" {
		q = q.Where("EXISTS (SELECT 1 FROM web3_user_addresses WHERE web3_user_addresses.web3_user_id = web3_users.id AND web3_user_addresses.is_primary = 1 AND web3_user_addresses.address LIKE ? AND web3_user_addresses.deleted_at IS NULL)", "%"+v+"%")
	}

	// Wallet count filter (requires subquery)
	if f.WalletCountMin != nil && *f.WalletCountMin > 0 {
		q = q.Where("(SELECT COUNT(*) FROM web3_user_addresses WHERE web3_user_addresses.web3_user_id = web3_users.id AND web3_user_addresses.deleted_at IS NULL) >= ?", *f.WalletCountMin)
	}
	if f.WalletCountMax != nil && *f.WalletCountMax > 0 {
		q = q.Where("(SELECT COUNT(*) FROM web3_user_addresses WHERE web3_user_addresses.web3_user_id = web3_users.id AND web3_user_addresses.deleted_at IS NULL) <= ?", *f.WalletCountMax)
	}

	// Keyword search (device_id or address)
	if v := strings.TrimSpace(f.Keyword); v != "" {
		q = q.Where("(device_id LIKE ? OR EXISTS (SELECT 1 FROM web3_user_addresses WHERE web3_user_addresses.web3_user_id = web3_users.id AND web3_user_addresses.address LIKE ? AND web3_user_addresses.deleted_at IS NULL))", "%"+v+"%", "%"+v+"%")
	}

	return q
}

type Web3UserAddressAgg struct {
	Web3UserID           int64  `gorm:"column:web3_user_id"`
	WalletCount          int64  `gorm:"column:wallet_count"`
	Networks             string `gorm:"column:networks"`
	PrimaryAddress       string `gorm:"column:primary_address"`
	HasBlacklisted       int64  `gorm:"column:has_blacklisted"`
	IsPrimaryBlacklisted int64  `gorm:"column:is_primary_blacklisted"`
}

type Web3UserAddressRepository interface {
	WithTx(tx *gorm.DB) Web3UserAddressRepository
	GetDB() *gorm.DB

	ListByUserID(ctx context.Context, userID int64) ([]*model.Web3UserAddressModel, error)
	FindByUserIDAndAddress(ctx context.Context, userID int64, address string) (*model.Web3UserAddressModel, error)
	AggregateByUserIDs(ctx context.Context, userIDs []int64) (map[int64]Web3UserAddressAgg, error)

	// New methods for GetWeb3UsersSummary
	CountAll(ctx context.Context) (int64, error)
	CountBlacklisted(ctx context.Context) (int64, error)

	// List for chainsync monitoring
	List(ctx context.Context, page, pageSize int32, network, status string) ([]*model.Web3UserAddressModel, int64, error)
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

func (r *web3UserAddressRepo) FindByUserIDAndAddress(ctx context.Context, userID int64, address string) (*model.Web3UserAddressModel, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("address not found")
	}
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, fmt.Errorf("address not found")
	}
	var m model.Web3UserAddressModel
	err := r.db.WithContext(ctx).
		Model(&model.Web3UserAddressModel{}).
		Where("web3_user_id = ? AND address = ? AND deleted_at IS NULL", userID, address).
		First(&m).Error
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
				"MAX(CASE WHEN is_blacklisted = 1 THEN 1 ELSE 0 END) AS has_blacklisted, "+
				"MAX(CASE WHEN is_primary = 1 AND is_blacklisted = 1 THEN 1 ELSE 0 END) AS is_primary_blacklisted",
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

// CountAll returns total count of all addresses
func (r *web3UserAddressRepo) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Web3UserAddressModel{}).
		Where("deleted_at IS NULL").
		Count(&count).Error
	return count, err
}

// CountBlacklisted returns count of blacklisted addresses
func (r *web3UserAddressRepo) CountBlacklisted(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Web3UserAddressModel{}).
		Where("deleted_at IS NULL AND is_blacklisted = 1").
		Count(&count).Error
	return count, err
}

// List returns paginated web3 user addresses for chainsync monitoring
func (r *web3UserAddressRepo) List(ctx context.Context, page, pageSize int32, network, status string) ([]*model.Web3UserAddressModel, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}

	query := r.db.WithContext(ctx).Model(&model.Web3UserAddressModel{}).
		Where("deleted_at IS NULL")

	// Filter by network
	network = strings.TrimSpace(network)
	if network != "" {
		query = query.Where("network = ?", network)
	}

	// Filter by status
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case "enabled":
		query = query.Where("enabled = 1 AND is_blacklisted = 0")
	case "disabled":
		query = query.Where("enabled = 0")
	case "blacklisted":
		query = query.Where("is_blacklisted = 1")
	case "":
		// No status filter, return all
	default:
		// Default: only enabled, non-blacklisted addresses
		query = query.Where("enabled = 1 AND is_blacklisted = 0")
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Query with pagination
	var items []*model.Web3UserAddressModel
	offset := int((page - 1) * pageSize)
	err := query.Order("id ASC").Offset(offset).Limit(int(pageSize)).Find(&items).Error

	return items, total, err
}
