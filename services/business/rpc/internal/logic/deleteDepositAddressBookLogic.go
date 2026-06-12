package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type DeleteDepositAddressBookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteDepositAddressBookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteDepositAddressBookLogic {
	return &DeleteDepositAddressBookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteDepositAddressBookLogic) DeleteDepositAddressBook(in *pb.DeleteDepositAddressBookReq) (*pb.DeleteDepositAddressBookResp, error) {
	if in == nil || strings.TrimSpace(in.Id) == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	id, err := strconv.ParseInt(strings.TrimSpace(in.Id), 10, 64)
	if err != nil || id <= 0 {
		return nil, errx.InvalidParam("invalid params")
	}

	if l.svcCtx.DB == nil || l.svcCtx.DepositAddressBookRepository == nil || l.svcCtx.WalletDepositRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	var (
		errNotFound        = errors.New("deposit address not found")
		errOnlyOne         = errors.New("cannot delete the only address")
		errHasPending      = errors.New("address has pending deposits")
		errPromoteNotFound = errors.New("no candidate for default")
	)

	txErr := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		addrRepo := l.svcCtx.DepositAddressBookRepository.WithTx(tx)
		depositRepo := l.svcCtx.WalletDepositRepository.WithTx(tx)

		addr, err := addrRepo.FindActiveByID(l.ctx, uid, id)
		if err != nil || addr == nil {
			return errNotFound
		}

		chainCode := strings.ToUpper(strings.TrimSpace(addr.ChainCode))
		activeCount, err := addrRepo.CountActiveByUserAndChain(l.ctx, uid, chainCode)
		if err != nil {
			return err
		}
		if activeCount <= 1 {
			return errOnlyOne
		}

		// Prevent deletion when there are pending/confirmed deposits for this address.
		pendingCount, err := depositRepo.CountByUserChainDepositAddressAndStatuses(l.ctx, uid, chainCode, addr.Address, []string{"pending", "confirmed"})
		if err != nil {
			return err
		}
		if pendingCount > 0 {
			return errHasPending
		}

		wasDefault := addr.IsDefault
		if err := addrRepo.SoftDelete(l.ctx, uid, id); err != nil {
			return err
		}

		// If deleting default, promote newest remaining as default.
		if wasDefault {
			var next model.WalletDepositAddressModel
			err := tx.WithContext(l.ctx).
				Model(&model.WalletDepositAddressModel{}).
				Where("user_id = ? AND chain_code = ? AND status = ? AND deleted_at IS NULL", uid, chainCode, "active").
				Order("id DESC").
				First(&next).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errPromoteNotFound
				}
				return err
			}
			if err := addrRepo.SetDefaultByID(l.ctx, uid, chainCode, next.ID); err != nil {
				return err
			}
		}

		return nil
	})

	switch {
	case txErr == nil:
		return &pb.DeleteDepositAddressBookResp{Success: true, Message: "ok"}, nil
	case errors.Is(txErr, errNotFound):
		return nil, errx.NotFound("deposit address not found")
	case errors.Is(txErr, errOnlyOne):
		return nil, errx.Conflict("cannot delete the only address for this chain")
	case errors.Is(txErr, errHasPending):
		return nil, errx.Conflict("address has pending deposits")
	case errors.Is(txErr, errPromoteNotFound):
		// Should not happen (we already checked activeCount > 1), but keep safe.
		return nil, errx.Internal("delete deposit address failed")
	default:
		l.Errorf("DeleteDepositAddressBook failed: %v", txErr)
		return nil, errx.Internal("delete deposit address failed")
	}

}
