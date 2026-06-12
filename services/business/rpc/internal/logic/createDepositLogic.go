package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/common/mq"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type CreateDepositLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateDepositLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateDepositLogic {
	return &CreateDepositLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateDepositLogic) CreateDeposit(in *pb.CreateDepositReq) (*pb.CreateDepositResp, error) {
	if in == nil || in.Asset == "" || in.Chain == "" {
		return nil, errx.InvalidParam("invalid params")
	}
	uidStr := middleware.GetUserID(l.ctx)
	if strings.TrimSpace(uidStr) == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)
	asset := strings.ToUpper(strings.TrimSpace(in.Asset))
	chain := strings.ToUpper(strings.TrimSpace(in.Chain))

	// Step 1: Check asset is enabled (from Accounting - source of truth).
	if l.svcCtx.AccountingRpc != nil {
		accResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: asset})
		if err != nil || accResp == nil || !accResp.Success || accResp.Item == nil || accResp.Item.Status != 1 {
			return nil, errx.AssetNotAvailable()
		}
	}

	// Step 2: Check asset-level Web3 deposit toggle (currency_settings.web3_deposit_enabled).
	if l.svcCtx.CurrencySettingsRepository != nil {
		if s, err := l.svcCtx.CurrencySettingsRepository.GetByAssetCode(l.ctx, asset); err == nil && s != nil {
			if !s.Web3DepositEnabled {
				return nil, errx.DepositDisabled()
			}
		}
	}

	// Step 3: Check chain is globally enabled (chain.status=1) and get chain info.
	var chainInfo *model.ChainModel
	if l.svcCtx.ChainRepository != nil {
		var err error
		chainInfo, err = l.svcCtx.ChainRepository.FindByName(l.ctx, chain)
		if err != nil || chainInfo == nil || chainInfo.Status != 1 {
			return nil, errx.NetworkNotAvailable()
		}
	}

	// Step 4: Check asset-chain mapping is enabled (currency_chain_settings.status=1 AND deposit_enabled=1).
	var chainSettings *model.CurrencyChainSettingsModel
	if l.svcCtx.CurrencyChainSettingsRepository != nil {
		if m, err := l.svcCtx.CurrencyChainSettingsRepository.FindByAssetChain(l.ctx, asset, chain); err == nil && m != nil {
			if m.Status != 1 || !m.DepositEnabled {
				return nil, errx.NetworkNotAvailable()
			}
			chainSettings = m
		}
	}

	// Step 5: Resolve user's deposit address for this chain.
	if l.svcCtx.WalletDepositAddressRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}
	if l.svcCtx.DB == nil || l.svcCtx.DepositAddressBookRepository == nil {
		return nil, errx.ServiceNotAvailable("db")
	}

	resolveDepositAddress := func() (*model.WalletDepositAddressModel, error) {
		if m, err := l.svcCtx.WalletDepositAddressRepository.FindActiveByUserAndChain(l.ctx, uid, chain); err == nil && m != nil {
			return m, nil
		}
		return nil, errx.NotFound("deposit address not found")
	}

	isDepositAddressNotFound := func(err error) bool {
		if err == nil {
			return false
		}
		// errx.NotFound wraps an internal message; be tolerant.
		msg := strings.ToLower(err.Error())
		return strings.Contains(msg, "deposit address not found") || strings.Contains(msg, "not found")
	}

	const defaultHotWalletSeedID = "hot_wallet_main"
	createDepositAddressIfMissing := func() (*model.WalletDepositAddressModel, error) {
		if l.svcCtx.SignerRpc == nil {
			return nil, errx.ServiceNotAvailable("signer")
		}
		now := time.Now()
		var created *model.WalletDepositAddressModel
		err := l.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
			repo := l.svcCtx.DepositAddressBookRepository.WithTx(tx)

			// Re-check within tx to keep it idempotent under concurrency.
			if m, err := l.svcCtx.WalletDepositAddressRepository.WithTx(tx).FindActiveByUserAndChain(l.ctx, uid, chain); err == nil && m != nil {
				created = m
				return nil
			}

			// Determine next derivation change index.
			totalAll, err := repo.CountByUserAndChain(l.ctx, uid, chain)
			if err != nil {
				return err
			}
			nextChange := 0
			setDefault := true
			if totalAll > 0 {
				maxChange, err := repo.GetMaxDerivationChange(l.ctx, uid, chain)
				if err != nil {
					// Production safety: DB schema might not have been migrated yet.
					if strings.Contains(err.Error(), "Unknown column 'derivation_change'") || strings.Contains(err.Error(), "derivation_change") {
						return errx.DBSchemaNotMigrated("deploy/docker/init-db/100-deposit-address-book.sql")
					}
					return err
				}
				nextChange = maxChange + 1
				setDefault = false
			}

			// Generate new address via Signer.
			signerResp, err := l.svcCtx.SignerRpc.GenerateUserDepositAddress(l.ctx, &pb.GenerateUserDepositAddressRequest{
				SeedId:      defaultHotWalletSeedID,
				UserId:      uid,
				Chain:       chain,
				ChangeIndex: int32(nextChange),
				Requester:   "business_create_deposit",
			})
			if err != nil || signerResp == nil || signerResp.Code != 200 || strings.TrimSpace(signerResp.Address) == "" {
				l.Errorf("GenerateUserDepositAddress failed: err=%v resp=%+v", err, signerResp)
				return errx.GenerateFailed()
			}
			address := strings.TrimSpace(signerResp.Address)

			m := &model.WalletDepositAddressModel{
				UserID:           uid,
				ChainCode:        chain,
				Address:          address,
				Status:           "active",
				Memo:             nil,
				Label:            nil,
				Remark:           nil,
				IsDefault:        setDefault,
				DerivationChange: nextChange,
				LastUsedAt:       nil,
				CreatedAt:        &now,
				UpdatedAt:        &now,
			}
			if err := repo.Create(l.ctx, m); err != nil {
				if strings.Contains(err.Error(), "Unknown column 'derivation_change'") || strings.Contains(err.Error(), "derivation_change") {
					return errx.DBSchemaNotMigrated("deploy/docker/init-db/100-deposit-address-book.sql")
				}
				return err
			}
			if setDefault {
				if err := repo.SetDefaultByID(l.ctx, uid, chain, m.ID); err != nil {
					return err
				}
				m.IsDefault = true
			}
			created = m
			return nil
		})
		if err != nil {
			// Best-effort: surface a clearer error instead of generic NOT_FOUND.
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, errx.ServiceNotAvailable("signer")
			}
			return nil, err
		}
		if created == nil {
			return nil, errx.Internal("create deposit address failed")
		}

		// Best-effort: publish address monitor event so ChainSync can immediately
		// add the new (non-default) deposit address to its in-memory registry.
		//
		// Without this, ChainSync only learns about newly created subaddresses via
		// periodic full refresh; deposits occurring shortly after address issuance
		// may be missed (no backfill).
		l.svcCtx.PublishAddressMonitorEvent(l.ctx, mq.AddressMonitorEvent{
			Action:  mq.AddressMonitorActionUpsert,
			Source:  mq.AddressMonitorSourceDeposit,
			Chain:   chain,
			Address: created.Address,
			Reason:  "deposit_address_created",
			Metadata: map[string]string{
				"user_id":   strconv.FormatInt(uid, 10),
				"chain":     chain,
				"address":   created.Address,
				"change":    strconv.Itoa(created.DerivationChange),
				"default":   strconv.FormatBool(created.IsDefault),
				"requester": "business_create_deposit",
			},
		})

		return created, nil
	}

	da, err := resolveDepositAddress()
	if err != nil {
		if isDepositAddressNotFound(err) {
			da, err = createDepositAddressIfMissing()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	address := strings.TrimSpace(da.Address)
	memo := ""
	if da.Memo != nil {
		memo = strings.TrimSpace(*da.Memo)
	}

	// Build response with enhanced fields for UI
	minDeposit := "0.01"
	minDepositText := "0.01 " + asset
	eta := "约10分钟"
	confirmations := int32(12)
	chainName := chain

	// Use chain info from earlier query
	if chainInfo != nil && strings.TrimSpace(chainInfo.Network) != "" {
		chainName = strings.TrimSpace(chainInfo.Network)
	}

	// Chain-specific defaults for ETA and confirmations
	switch strings.ToUpper(chain) {
	case "TRON":
		eta = "约1分钟"
		confirmations = 1
	case "BSC":
		eta = "约1分钟"
		confirmations = 3
	case "ETH":
		eta = "约1分钟"
		confirmations = 12
	case "BTC":
		eta = "约30分钟"
		confirmations = 3
	}

	// Use per-asset-chain settings from earlier query (already loaded as chainSettings)
	if chainSettings != nil && chainSettings.MinDepositAmount != nil && strings.TrimSpace(*chainSettings.MinDepositAmount) != "" {
		minDeposit = strings.TrimSpace(*chainSettings.MinDepositAmount)
		minDepositText = minDeposit + " " + asset
	}

	// Build tips list
	tipsList := []string{
		"请勿向上述地址充值任何非" + asset + "资产，否则资产将不可找回",
		"最小充值金额为" + minDepositText + "，小于最小金额的充值将不会上账",
		"您的充值地址不会经常改变，可以重复充值",
	}

	return &pb.CreateDepositResp{
		Success:               true,
		Address:               address,
		MemoTag:               memo,
		QrcodeUrl:             "",
		QrcodeData:            address,
		Asset:                 asset,
		Chain:                 chain,
		ChainName:             chainName,
		MinDeposit:            minDeposit,
		MinDepositText:        minDepositText,
		ConfirmationsRequired: confirmations,
		Eta:                   eta,
		Tips:                  "请确保选择正确的网络，否则可能导致资产丢失",
		TipsList:              tipsList,
	}, nil
}
