package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWithdrawRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWithdrawRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWithdrawRecordsLogic {
	return &ListWithdrawRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListWithdrawRecordsLogic) ListWithdrawRecords(in *pb.ListWithdrawRecordsReq) (*pb.ListWithdrawRecordsResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	if uidStr == "" {
		return nil, errx.Unauthorized("unauthorized")
	}
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	// Parse pagination
	page := int32(1)
	pageSize := int32(20)
	if in != nil {
		if in.Page > 0 {
			page = in.Page
		}
		if in.PageSize > 0 {
			pageSize = in.PageSize
		}
	}

	items := []*pb.WithdrawRecordItem{}

	if l.svcCtx.AccountingRpc == nil {
		return &pb.ListWithdrawRecordsResp{Success: true, Total: 0, Items: items}, nil
	}

	req := &pb.ListUserTransactionRecordsRequest{
		Page:     page,
		PageSize: pageSize,
		UserId:   uid,
		TxType:   2, // withdraw
	}
	if in != nil {
		// Asset filter
		if asset := strings.TrimSpace(in.Asset); asset != "" {
			req.AssetCode = asset
		}
		// Chain/Network filter
		if chain := strings.TrimSpace(in.Chain); chain != "" {
			req.ChainCode = chain
		}
		// Status filter (supports comma-separated; Accounting will interpret as IN)
		if status := strings.TrimSpace(in.Status); status != "" {
			req.Status = status
		}
		// Time range filter
		if in.StartTime > 0 {
			req.StartTime = in.StartTime
		}
		if in.EndTime > 0 {
			req.EndTime = in.EndTime
		}
	}

	accResp, err := l.svcCtx.AccountingRpc.ListUserTransactionRecords(l.ctx, req)
	if err != nil || accResp == nil || !accResp.Success {
		l.Logger.Errorf("ListUserTransactionRecords failed: %v", err)
		return &pb.ListWithdrawRecordsResp{Success: true, Total: 0, Items: items}, nil
	}

	// Pre-load asset metadata (icon URLs) from Accounting service
	assetIconMap := make(map[string]string)
	if l.svcCtx.AccountingRpc != nil {
		listResp, err := l.svcCtx.AccountingRpc.ListAssets(l.ctx, &pb.ListAssetsRequest{
			Page:     1,
			PageSize: 100,
			Status:   1, // enabled only
		})
		if err != nil {
			l.Logger.Errorf("ListAssets failed: %v", err)
		} else if listResp != nil {
			for _, asset := range listResp.GetItems() {
				if asset == nil {
					continue
				}
				code := strings.ToUpper(strings.TrimSpace(asset.Code))
				if code != "" {
					assetIconMap[code] = strings.TrimSpace(asset.IconUrl)
				}
			}
		}
	}

	// Pre-load chain info (batch for efficiency)
	chainInfoCache := make(map[string]*struct {
		displayName   string
		explorerURL   string
		confirmations int32
	})
	chainSet := make(map[string]bool)
	for _, r := range accResp.GetItems() {
		if r == nil {
			continue
		}
		if strings.TrimSpace(r.ChainCode) != "" {
			chainSet[strings.ToUpper(r.ChainCode)] = true
		}
	}
	for chainCode := range chainSet {
		chain := GetChainInfo(l.ctx, l.svcCtx, chainCode)
		displayName := MapChainDisplayName(chainCode)
		if chain != nil && strings.TrimSpace(chain.Network) != "" {
			displayName = strings.TrimSpace(chain.Network)
		}
		chainInfoCache[chainCode] = &struct {
			displayName   string
			explorerURL   string
			confirmations int32
		}{
			displayName:   displayName,
			explorerURL:   "",
			confirmations: GetRequiredConfirmations(chainCode),
		}
		if chain != nil {
			chainInfoCache[chainCode].explorerURL = strings.TrimSpace(chain.ExplorerUrl)
		}
	}

	// Build response items
	for _, r := range accResp.GetItems() {
		if r == nil {
			continue
		}
		asset := strings.ToUpper(strings.TrimSpace(r.AssetCode))
		chainCode := strings.ToUpper(strings.TrimSpace(r.ChainCode))
		txHash := strings.TrimSpace(r.TxHash)

		// Get chain info from cache
		chainInfo := chainInfoCache[chainCode]
		chainName := chainCode
		explorerURL := ""
		requiredConfs := GetRequiredConfirmations(chainCode)
		if chainInfo != nil {
			chainName = chainInfo.displayName
			requiredConfs = chainInfo.confirmations
			if chainInfo.explorerURL != "" && txHash != "" {
				if strings.Contains(chainInfo.explorerURL, "%s") {
					explorerURL = strings.Replace(chainInfo.explorerURL, "%s", txHash, 1)
				} else {
					explorerURL = strings.TrimSuffix(chainInfo.explorerURL, "/") + "/tx/" + txHash
				}
			}
		}
		if explorerURL == "" && txHash != "" {
			explorerURL = BuildExplorerURL(chainCode, txHash)
		}

		// Get from_address and to_address
		fromAddress := strings.TrimSpace(r.FromAddress)
		toAddress := strings.TrimSpace(r.ToAddress)

		// Try to get more details from withdraw order if available
		if l.svcCtx.CurrencyWithdrawOrderRepository != nil {
			if order, err := l.svcCtx.CurrencyWithdrawOrderRepository.FindByUserIDAndID(l.ctx, uid, r.Id); err == nil && order != nil {
				if fromAddress == "" {
					fromAddress = strings.TrimSpace(order.FromAddress)
				}
				if toAddress == "" && strings.TrimSpace(order.ToAddress) != "" {
					toAddress = strings.TrimSpace(order.ToAddress)
				}
				if txHash == "" && order.TxHash != nil && strings.TrimSpace(*order.TxHash) != "" {
					txHash = strings.TrimSpace(*order.TxHash)
				}
			}
		}
		// If tx_hash was filled from order, rebuild explorer URL
		if explorerURL == "" && txHash != "" {
			if chainInfo != nil && chainInfo.explorerURL != "" {
				if strings.Contains(chainInfo.explorerURL, "%s") {
					explorerURL = strings.Replace(chainInfo.explorerURL, "%s", txHash, 1)
				} else {
					explorerURL = strings.TrimSuffix(chainInfo.explorerURL, "/") + "/tx/" + txHash
				}
			}
		}
		if explorerURL == "" && txHash != "" {
			explorerURL = BuildExplorerURL(chainCode, txHash)
		}

		// Format timestamp
		createdAt := ""
		processedAt := ""
		if r.Timestamp > 0 {
			createdAt = time.Unix(r.Timestamp, 0).Format("2006/01/02 15:04:05")
		}

		// Determine confirmations
		confirmations := int32(0)
		status := strings.ToLower(strings.TrimSpace(r.Status))
		if status == "success" || status == "completed" {
			confirmations = requiredConfs
			processedAt = createdAt
		}

		// Fee info
		fee := strings.TrimSpace(r.FeeDecimal)
		feeAsset := asset

		// Get asset icon URL
		iconUrl := assetIconMap[asset]

		items = append(items, &pb.WithdrawRecordItem{
			Id:                    strconv.FormatInt(r.Id, 10),
			Asset:                 asset,
			Chain:                 chainCode,
			ChainName:             chainName,
			Amount:                strings.TrimSpace(r.AmountDecimal),
			AmountDisplay:         FormatAmountDisplay(r.AmountDecimal, asset, false),
			FromAddress:           fromAddress,
			ToAddress:             toAddress,
			TxHash:                txHash,
			ExplorerUrl:           explorerURL,
			Status:                r.Status,
			StatusText:            MapStatusText(r.Status),
			Fee:                   fee,
			FeeAsset:              feeAsset,
			Confirmations:         confirmations,
			RequiredConfirmations: requiredConfs,
			CreatedAt:             createdAt,
			ProcessedAt:           processedAt,
			IconUrl:               iconUrl,
		})
	}

	return &pb.ListWithdrawRecordsResp{Success: true, Total: int32(accResp.Total), Items: items}, nil
}
