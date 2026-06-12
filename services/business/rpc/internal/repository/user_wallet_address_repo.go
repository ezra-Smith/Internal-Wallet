package repository

import (
	"context"

	commonRepo "internalwallet/common/repository"
	"internalwallet/services/business/rpc/internal/model"

	"gorm.io/gorm"
)

type UserWalletAddressRepository interface {
	commonRepo.BaseRepository[model.UserWalletAddressModel]
	CreateDefaultAddress(ctx context.Context, userID int64, asset string, chain string) error
	CreateAddress(ctx context.Context, addr *model.UserWalletAddressModel) error
	SetAllDefaultFalseByUser(ctx context.Context, userID int64) error
	ListAddresses(ctx context.Context, userID int64, asset string, chain string) ([]model.UserWalletAddressModel, error)
}

type userWalletAddressRepo struct {
	commonRepo.BaseRepository[model.UserWalletAddressModel]
}

func NewUserWalletAddressRepository(db *gorm.DB) UserWalletAddressRepository {
	return &userWalletAddressRepo{
		BaseRepository: commonRepo.NewBaseRepository[model.UserWalletAddressModel](db),
	}
}

func (r *userWalletAddressRepo) CreateDefaultAddress(ctx context.Context, userID int64, asset string, chain string) error {
	return r.GetDB().WithContext(context.Background()).Create(&model.UserWalletAddressModel{
		UserId:    userID,
		Asset:     asset,
		Chain:     chain,
		Address:   "",
		Label:     "Default",
		IsDefault: true,
	}).Error
}

func (r *userWalletAddressRepo) CreateAddress(ctx context.Context, addr *model.UserWalletAddressModel) error {
	return r.GetDB().WithContext(context.Background()).Create(addr).Error
}

func (r *userWalletAddressRepo) SetAllDefaultFalseByUser(ctx context.Context, userID int64) error {
	return r.GetDB().WithContext(context.Background()).Model(&model.UserWalletAddressModel{}).Where("user_id = ?", userID).Updates(map[string]interface{}{"is_default": false}).Error
}

func (r *userWalletAddressRepo) ListAddresses(ctx context.Context, userID int64, asset string, chain string) ([]model.UserWalletAddressModel, error) {
	q := r.GetDB().WithContext(context.Background()).Model(&model.UserWalletAddressModel{})
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if asset != "" {
		// 链上地址可以接收该链的任何代币，所以同时匹配指定 asset 和 asset 为空的记录
		q = q.Where("asset = ? OR asset = ''", asset)
	}
	if chain != "" {
		// 映射代币代码到链代码（前端可能传代币代码如 TRX/BNB，需要映射到链代码 TRON/BSC）
		chainCode := normalizeChainCode(chain)
		q = q.Where("chain = ?", chainCode)
	}
	var rows []model.UserWalletAddressModel
	err := q.Order("created_at DESC").Find(&rows).Error
	return rows, err
}

// normalizeChainCode 将代币代码映射到链代码
func normalizeChainCode(chain string) string {
	switch chain {
	case "TRX", "trx", "TRON", "tron", "TRC20", "trc20":
		return "TRON"
	case "BNB", "bnb", "BSC", "bsc", "BEP20", "bep20":
		return "BSC"
	case "ETH", "eth", "ERC20", "erc20", "Ethereum", "ethereum":
		return "ETH"
	default:
		return chain
	}
}
