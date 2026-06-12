package logic

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	admininterceptor "internalwallet/services/admin/rpc/internal/interceptor"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreateCurrencyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCurrencyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCurrencyLogic {
	return &CreateCurrencyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateCurrencyLogic) CreateCurrency(in *pb.CreateCurrencyRequest) (*pb.CreateCurrencyResponse, error) {
	if in == nil || strings.TrimSpace(in.AssetCode) == "" {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PARAM", "invalid params", map[string]string{
			"asset_code": "required",
		})
	}
	if l.svcCtx.DB == nil || l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	current, ok := admininterceptor.GetCurrentAdmin(l.ctx)
	if !ok || current == nil {
		return nil, errx.New(codes.Unauthenticated, 401, errx.CodeUnauthorized, "AUTH_TOKEN_INVALID", "unauthorized", nil)
	}

	assetCode := normalizeCode(in.AssetCode)
	if !isValidAssetCode(assetCode) {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ASSET_CODE", "invalid asset_code", map[string]string{
			"asset_code": "invalid",
		})
	}

	assetName := strings.TrimSpace(in.AssetName)
	if assetName == "" {
		assetName = assetCode
	}
	if len(assetName) > 64 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ASSET_NAME", "invalid asset_name", map[string]string{
			"asset_name": "too long",
		})
	}

	precision := int32(18)
	if in.Precision != nil {
		precision = in.Precision.GetValue()
	}
	if precision < 0 || precision > 30 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_PRECISION", "invalid precision", map[string]string{
			"precision": "invalid",
		})
	}

	status := int32(2)
	if in.Status != nil {
		status = in.Status.GetValue()
	}
	if status != 1 && status != 2 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{
			"status": "invalid",
		})
	}

	iconURL := ""
	if in.IconUrl != nil {
		iconURL = strings.TrimSpace(in.IconUrl.GetValue())
	}
	if iconURL != "" {
		if len(iconURL) > 2048 {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ICON_URL", "icon_url too long", map[string]string{"icon_url": "too long"})
		}
		u, parseErr := url.ParseRequestURI(iconURL)
		if parseErr != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_ICON_URL", "invalid icon_url", map[string]string{"icon_url": "invalid"})
		}
	}

	// If currency settings already exist, treat it as already configured.
	var existing model.CurrencySettingsModel
	if err := l.svcCtx.DB.WithContext(l.ctx).
		Where("asset_code = ?", assetCode).
		First(&existing).Error; err == nil {
		return nil, errx.New(codes.AlreadyExists, 409, errx.CodeConflict, "ALREADY_EXISTS", "asset already exists", map[string]string{
			"asset_code": "exists",
		})
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		l.Logger.Errorf("check currency_settings failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	// Asset is managed by Accounting (source of truth).
	assetItem := (*pb.AcctAsset)(nil)
	accResp, callErr := l.svcCtx.AccountingRpc.CreateAsset(l.ctx, &pb.CreateAssetRequest{
		Code:      assetCode,
		Name:      assetName,
		Precision: precision,
		Status:    status,
		IconUrl:   iconURL,
	})
	if callErr != nil {
		l.Logger.Errorf("call accounting CreateAsset failed: %v", callErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if accResp != nil && accResp.Success && accResp.Item != nil {
		assetItem = accResp.Item
	}
	if assetItem == nil {
		// Best-effort recovery (e.g. duplicate asset created on a previous attempt).
		getResp, getErr := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: assetCode})
		if getErr != nil {
			l.Logger.Errorf("call accounting GetAsset failed: %v", getErr)
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
		}
		if getResp == nil || !getResp.Success || getResp.Item == nil {
			return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
		}
		assetItem = getResp.Item
	}

	settings := &model.CurrencySettingsModel{
		AssetCode:              assetCode,
		Web2DepositEnabled:     false,
		Web2WithdrawEnabled:    false,
		Web2TransferEnabled:    false,
		Web3DepositEnabled:     false,
		Web3WithdrawEnabled:    false,
		Web3SwapEnabled:        enforceBaseCurrencySwapEnabled(assetCode, false),
		UseGlobalWithdrawFee:   true,
		UseGlobalWithdrawAudit: true,
		UpdatedBy:              current.ID,
	}

	chainTotal := int32(0)
	chainEnabled := int32(0)
	updatedTime := ""
	if err := l.svcCtx.DB.WithContext(l.ctx).Transaction(func(tx *gorm.DB) error {
		if l.svcCtx.CurrencySettingsRepo != nil {
			if err := l.svcCtx.CurrencySettingsRepo.WithTx(tx).Upsert(l.ctx, settings); err != nil {
				return err
			}
		}

		if l.svcCtx.ChainRepo != nil {
			chains, err := l.svcCtx.ChainRepo.WithTx(tx).ListEnabled(l.ctx)
			if err != nil {
				return err
			}
			if len(chains) > 0 {
				items := make([]*model.CurrencyChainSettingsModel, 0, len(chains))
				for _, c := range chains {
					if c == nil {
						continue
					}
					cc := normalizeCode(c.Name)
					if cc == "" {
						continue
					}
					items = append(items, &model.CurrencyChainSettingsModel{
						AssetCode:       assetCode,
						ChainCode:       cc,
						DepositEnabled:  false,
						WithdrawEnabled: false,
						Status:          2,
						UpdatedBy:       current.ID,
					})
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&items).Error; err != nil {
					return err
				}
				chainTotal = int32(len(items))
			}
		}

		updatedTime = formatTime(settings.UpdatedAt)
		return nil
	}); err != nil {
		l.Logger.Errorf("create currency failed: %v", err)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}

	settingsPB := &pb.CurrencyFeatureSettings{
		Web2DepositEnabled:     settings.Web2DepositEnabled,
		Web2WithdrawEnabled:    settings.Web2WithdrawEnabled,
		Web2TransferEnabled:    settings.Web2TransferEnabled,
		Web3DepositEnabled:     settings.Web3DepositEnabled,
		Web3WithdrawEnabled:    settings.Web3WithdrawEnabled,
		Web3SwapEnabled:        enforceBaseCurrencySwapEnabled(assetCode, settings.Web3SwapEnabled),
		UseGlobalWithdrawFee:   settings.UseGlobalWithdrawFee,
		UseGlobalWithdrawAudit: settings.UseGlobalWithdrawAudit,
	}

	return &pb.CreateCurrencyResponse{
		Success: true,
		Message: "ok",
		Data: &pb.CreateCurrencyData{
			Currency: &pb.CurrencyItem{
				AssetCode: assetCode,
				AssetName: func() string {
					name := strings.TrimSpace(assetItem.Name)
					if name == "" {
						name = assetCode
					}
					return name
				}(),
				Status:       assetItem.Status,
				Settings:     settingsPB,
				ChainTotal:   chainTotal,
				ChainEnabled: chainEnabled,
				UpdatedAt:    updatedTime,
				IconUrl:      strings.TrimSpace(assetItem.IconUrl),
			},
		},
		RequestId: resp.RequestID(l.ctx),
		Timestamp: resp.Timestamp(),
	}, nil
}

func isValidAssetCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || len(code) > 32 {
		return false
	}
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
