package logic

import (
	"context"
	"fmt"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/admin/rpc/internal/errx"
	"internalwallet/services/admin/rpc/internal/model"
	"internalwallet/services/admin/rpc/internal/resp"
	"internalwallet/services/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
)

type ListCurrenciesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCurrenciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCurrenciesLogic {
	return &ListCurrenciesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListCurrenciesLogic) ListCurrencies(in *pb.ListCurrenciesRequest) (*pb.ListCurrenciesResponse, error) {
	if in == nil {
		in = &pb.ListCurrenciesRequest{}
	}
	if l.svcCtx.DB == nil || l.svcCtx.AccountingRpc == nil {
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "db not configured", nil)
	}

	status := in.Status
	if status != 0 && status != 1 && status != 2 {
		return nil, errx.New(codes.InvalidArgument, 400, errx.CodeInvalidParam, "INVALID_STATUS", "invalid status", map[string]string{"status": "invalid"})
	}

	accResp, callErr := l.svcCtx.AccountingRpc.ListAssets(l.ctx, &pb.ListAssetsRequest{
		Page:     in.Page,
		PageSize: in.PageSize,
		Keyword:  in.Keyword,
		Status:   status,
	})
	if callErr != nil {
		l.Logger.Errorf("call accounting ListAssets failed: %v", callErr)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "accounting service error", nil)
	}
	if accResp == nil || !accResp.Success {
		msg := ""
		if accResp != nil {
			msg = accResp.Message
		}
		l.Logger.Errorf("accounting ListAssets not success: %s", msg)
		return nil, errx.New(codes.Internal, 500, errx.CodeInternalError, "INTERNAL", "internal error", nil)
	}
	assets := accResp.Items
	total := accResp.Total
	if len(assets) == 0 {
		return &pb.ListCurrenciesResponse{
			Success: true,
			Message: "ok (0)",
			Data: &pb.ListCurrenciesData{
				Currencies: []*pb.CurrencyItem{},
			},
			Pagination: calcPagination(in.Page, in.PageSize, total),
			RequestId:  resp.RequestID(l.ctx),
			Timestamp:  resp.Timestamp(),
		}, nil
	}

	assetCodes := make([]string, 0, len(assets))
	for _, a := range assets {
		if a == nil || strings.TrimSpace(a.Code) == "" {
			continue
		}
		assetCodes = append(assetCodes, strings.ToUpper(strings.TrimSpace(a.Code)))
	}

	// Settings (batch)
	var settingsRows []*model.CurrencySettingsModel
	_ = l.svcCtx.DB.WithContext(l.ctx).
		Where("asset_code IN ?", assetCodes).
		Find(&settingsRows).Error
	settingsByAsset := make(map[string]*model.CurrencySettingsModel, len(settingsRows))
	for _, s := range settingsRows {
		if s == nil {
			continue
		}
		settingsByAsset[strings.ToUpper(strings.TrimSpace(s.AssetCode))] = s
	}

	// Chain counts (batch)
	type chainCountRow struct {
		AssetCode string
		Total     int64
		Enabled   int64
	}
	var chainCounts []chainCountRow
	_ = l.svcCtx.DB.WithContext(l.ctx).
		Table("currency_chain_settings").
		Select("asset_code AS asset_code, COUNT(*) AS total, SUM(CASE WHEN status = 1 AND deleted_at IS NULL THEN 1 ELSE 0 END) AS enabled").
		Where("deleted_at IS NULL AND asset_code IN ?", assetCodes).
		Group("asset_code").
		Scan(&chainCounts).Error
	chainCountByAsset := make(map[string]chainCountRow, len(chainCounts))
	for _, r := range chainCounts {
		chainCountByAsset[strings.ToUpper(strings.TrimSpace(r.AssetCode))] = r
	}

	respItems := make([]*pb.CurrencyItem, 0, len(assets))
	for _, a := range assets {
		if a == nil || strings.TrimSpace(a.Code) == "" {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(a.Code))
		s := settingsByAsset[code]

		settingsPB := &pb.CurrencyFeatureSettings{
			Web2DepositEnabled:     false,
			Web2WithdrawEnabled:    false,
			Web2TransferEnabled:    false,
			Web3DepositEnabled:     true,
			Web3WithdrawEnabled:    true,
			Web3SwapEnabled:        false,
			UseGlobalWithdrawFee:   true,
			UseGlobalWithdrawAudit: true,
			UseGlobalTransferAudit: true, // 默认值：使用全局规则
		}
		updatedTime := ""
		if s != nil {
			settingsPB = &pb.CurrencyFeatureSettings{
				Web2DepositEnabled:     s.Web2DepositEnabled,
				Web2WithdrawEnabled:    s.Web2WithdrawEnabled,
				Web2TransferEnabled:    s.Web2TransferEnabled,
				Web3DepositEnabled:     s.Web3DepositEnabled,
				Web3WithdrawEnabled:    s.Web3WithdrawEnabled,
				Web3SwapEnabled:        enforceBaseCurrencySwapEnabled(code, s.Web3SwapEnabled),
				UseGlobalWithdrawFee:   s.UseGlobalWithdrawFee,
				UseGlobalWithdrawAudit: s.UseGlobalWithdrawAudit,
				UseGlobalTransferAudit: s.UseGlobalTransferAudit, // 从数据库读取
			}
			updatedTime = formatTime(s.UpdatedAt)
		} else {
			settingsPB.Web3SwapEnabled = enforceBaseCurrencySwapEnabled(code, settingsPB.Web3SwapEnabled)
		}

		cc := chainCountByAsset[code]
		name := strings.TrimSpace(a.Name)
		if name == "" {
			name = code
		}
		respItems = append(respItems, &pb.CurrencyItem{
			AssetCode:    code,
			AssetName:    name,
			Status:       a.Status,
			Settings:     settingsPB,
			ChainTotal:   int32(cc.Total),
			ChainEnabled: int32(cc.Enabled),
			UpdatedAt:    updatedTime,
			IconUrl:      strings.TrimSpace(a.IconUrl),
			Precision:    a.Precision,
		})
	}

	p := calcPagination(in.Page, in.PageSize, total)
	return &pb.ListCurrenciesResponse{
		Success: true,
		Message: fmt.Sprintf("ok (%d)", total),
		Data: &pb.ListCurrenciesData{
			Currencies: respItems,
		},
		Pagination: p,
		RequestId:  resp.RequestID(l.ctx),
		Timestamp:  resp.Timestamp(),
	}, nil
}
