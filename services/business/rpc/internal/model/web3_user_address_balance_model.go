package model

import "time"

// Web3UserAddressBalanceModel maps to web3_user_address_balances
// Web3用户地址余额表（支持主币和Token）
// 注意：
//  1. 已删除 active_key 生成列，改用普通唯一索引 uk_web3_address_balance
//  2. 复合索引在 SQL 中定义，GORM 的 index 标签只用于单列索引
//  3. 已删除冗余单列索引（is_spam, is_verified, sync_status, balance_raw, balance_updated_at, last_synced_at）
//     这些字段在复合索引中，查询时会被覆盖
//  4. 余额记录通常只更新不删除，如果业务需要软删除后重新创建，可以恢复 active_key 模式
//  5. 已删除 web3_user_address_id 和 web3_user_id 字段，余额记录是地址级别的，不绑定到特定用户/设备
//     唯一索引为 (asset_code, chain_code, wallet_address)
type Web3UserAddressBalanceModel struct {
	ID int64 `gorm:"column:id;primaryKey;autoIncrement"`

	// 币种和链信息（关联到后台的 asset 和 chain 表）
	// 注意：asset_code 和 chain_code 在复合索引 idx_asset_chain 中
	AssetCode string `gorm:"column:asset_code;type:varchar(32);not null"`
	ChainCode string `gorm:"column:chain_code;type:varchar(32);not null"`

	// 地址信息（冗余，便于查询）
	WalletAddress string `gorm:"column:wallet_address;type:varchar(255);not null;index:idx_wallet_address"`
	Network       string `gorm:"column:network;type:varchar(50);not null"`
	ChainID       int64  `gorm:"column:chain_id;type:bigint;not null;default:0"`

	// 余额字段
	Balance       string `gorm:"column:balance;type:varchar(100);not null;default:'0'"`
	BalanceRaw    string `gorm:"column:balance_raw;type:decimal(65,0);not null;default:0"` // DECIMAL(65,0) in DB; keep as string to avoid int64 overflow. 已删除单列索引，在复合索引中
	BalanceUSD    string `gorm:"column:balance_usd;type:varchar(100);not null;default:'0'"`
	BalanceUSDRaw string `gorm:"column:balance_usd_raw;type:decimal(65,0);not null;default:0;index:idx_balance_usd_raw"` // DECIMAL(65,0) in DB; keep as string to avoid int64 overflow

	// Token 信息
	TokenAddress  *string `gorm:"column:token_address;type:varchar(255)"`
	TokenDecimals int     `gorm:"column:token_decimals;type:int;not null;default:18"`
	TokenType     string  `gorm:"column:token_type;type:varchar(20);not null;default:'native'"`

	// 价格和 Logo 从 accounting/market 服务实时获取，不在数据库中缓存

	// 元数据（用于过滤）
	// 注意：is_spam 和 is_verified 在复合索引 idx_wallet_address_spam_verified 中
	IsVerified int8 `gorm:"column:is_verified;type:tinyint(1);not null;default:0"`
	IsSpam     int8 `gorm:"column:is_spam;type:tinyint(1);not null;default:0"`

	// 同步相关
	// 注意：sync_status 和 last_synced_at 在复合索引 idx_sync_status_synced_at 中
	BalanceUpdatedAt *time.Time `gorm:"column:balance_updated_at;type:timestamp(6)"`                    // 已删除单列索引
	LastSyncedAt     *time.Time `gorm:"column:last_synced_at;type:timestamp(6)"`                        // 在复合索引中
	SyncStatus       string     `gorm:"column:sync_status;type:varchar(20);not null;default:'unknown'"` // 在复合索引中
	SyncError        *string    `gorm:"column:sync_error;type:varchar(512)"`

	// 时间戳（SQL 中为 NOT NULL，使用 time.Time 非空类型，GORM 自动处理）
	CreatedAt time.Time  `gorm:"column:created_at;type:timestamp(6);autoCreateTime"`
	UpdatedAt time.Time  `gorm:"column:updated_at;type:timestamp(6);autoUpdateTime"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:timestamp(6);index:idx_deleted_at"`
}

func (Web3UserAddressBalanceModel) TableName() string {
	return "web3_user_address_balances"
}
