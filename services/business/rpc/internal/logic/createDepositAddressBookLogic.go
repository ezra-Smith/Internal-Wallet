package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type CreateDepositAddressBookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateDepositAddressBookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateDepositAddressBookLogic {
	return &CreateDepositAddressBookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

const defaultHotWalletSeedID = "hot_wallet_main"

func (l *CreateDepositAddressBookLogic) CreateDepositAddressBook(in *pb.CreateDepositAddressBookReq) (*pb.CreateDepositAddressBookResp, error) {
	if in == nil || strings.TrimSpace(in.Chain) == "" {
		return nil, errx.InvalidParam("invalid params")
	}

	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	chainCode := strings.ToUpper(strings.TrimSpace(in.Chain))

	if l.svcCtx.DB == nil || l.svcCtx.DepositAddressBookRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}
	if l.svcCtx.SignerRpc == nil {
		return nil, errx.ServiceNotAvailable("signer")
	}

	// Validate chain is enabled
	var chainInfo *model.ChainModel
	if l.svcCtx.ChainRepository != nil {
		if info, err := l.svcCtx.ChainRepository.FindByName(l.ctx, chainCode); err == nil && info != nil && info.Status == 1 {
			chainInfo = info
		} else {
			return nil, errx.NetworkNotAvailable()
		}
	}

	// Limit: max 20 active addresses per chain per user
	activeCount, err := l.svcCtx.DepositAddressBookRepository.CountActiveByUserAndChain(l.ctx, uid, chainCode)
	if err != nil {
		l.Errorf("CountActiveByUserAndChain failed: %v", err)
		return nil, errx.Internal("query deposit addresses failed")
	}
	if activeCount >= 20 {
		return nil, errx.Conflict("address limit reached")
	}

	// Determine next derivation change index.
	totalAll, err := l.svcCtx.DepositAddressBookRepository.CountByUserAndChain(l.ctx, uid, chainCode)
	if err != nil {
		l.Errorf("CountByUserAndChain failed: %v", err)
		return nil, errx.Internal("query deposit addresses failed")
	}
	nextChange := 0
	setDefault := in.SetDefault
	if totalAll > 0 {
		maxChange, err := l.svcCtx.DepositAddressBookRepository.GetMaxDerivationChange(l.ctx, uid, chainCode)
		if err != nil {
			l.Errorf("GetMaxDerivationChange failed: %v", err)
			return nil, errx.Internal("query deposit addresses failed")
		}
		nextChange = maxChange + 1
	} else {
		// First address on this chain must be default (change=0).
		setDefault = true
	}

	// Generate new address via Signer.
	signerResp, err := l.svcCtx.SignerRpc.GenerateUserDepositAddress(l.ctx, &pb.GenerateUserDepositAddressRequest{
		SeedId:      defaultHotWalletSeedID,
		UserId:      uid,
		Chain:       chainCode,
		ChangeIndex: int32(nextChange),
		Requester:   "business_deposit_address_book_create",
	})
	if err != nil || signerResp == nil || signerResp.Code != 200 || strings.TrimSpace(signerResp.Address) == "" {
		l.Errorf("GenerateUserDepositAddress failed: err=%v resp=%+v", err, signerResp)
		return nil, errx.GenerateFailed()
	}
	address := strings.TrimSpace(signerResp.Address)

	now := time.Now()
	var created *model.WalletDepositAddressModel
	if err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		repo := l.svcCtx.DepositAddressBookRepository.WithTx(tx)

		m := &model.WalletDepositAddressModel{
			UserID:           uid,
			ChainCode:        chainCode,
			Address:          address,
			Status:           "active",
			Memo:             nil,
			Label:            trimToNil(in.Label),
			Remark:           trimToNil(in.Remark),
			IsDefault:        false,
			DerivationChange: nextChange,
			LastUsedAt:       nil,
			CreatedAt:        &now,
			UpdatedAt:        &now,
		}
		if err := repo.Create(l.ctx, m); err != nil {
			return err
		}
		created = m

		if setDefault {
			if err := repo.SetDefaultByID(l.ctx, uid, chainCode, created.ID); err != nil {
				return err
			}
			created.IsDefault = true
		}
		return nil
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, errx.Conflict("duplicate address")
		}
		l.Errorf("CreateDepositAddressBook tx failed: %v", err)
		return nil, errx.Internal("create deposit address failed")
	}

	chainName := MapChainDisplayName(chainCode)
	iconURL := ""
	if chainInfo != nil {
		if strings.TrimSpace(chainInfo.Network) != "" {
			chainName = strings.TrimSpace(chainInfo.Network)
		} else if strings.TrimSpace(chainInfo.Name) != "" {
			chainName = strings.TrimSpace(chainInfo.Name)
		}
		iconURL = strings.TrimSpace(chainInfo.IconUrl)
	}

	label := ""
	if created.Label != nil {
		label = strings.TrimSpace(*created.Label)
	}
	remark := ""
	if created.Remark != nil {
		remark = strings.TrimSpace(*created.Remark)
	}

	return &pb.CreateDepositAddressBookResp{
		Success: true,
		Message: "ok",
		Item: &pb.DepositAddressBookItem{
			Id:               strconv.FormatInt(created.ID, 10),
			Chain:            chainCode,
			ChainName:        chainName,
			IconUrl:          iconURL,
			Address:          strings.TrimSpace(created.Address),
			Label:            label,
			Remark:           remark,
			IsDefault:        created.IsDefault,
			DerivationChange: int32(created.DerivationChange),
			CreatedAt:        formatTimePtr(created.CreatedAt),
			LastUsedAt:       formatTimePtr(created.LastUsedAt),
			TotalReceived:    "",
			ReceivedCount:    0,
		},
	}, nil

}
