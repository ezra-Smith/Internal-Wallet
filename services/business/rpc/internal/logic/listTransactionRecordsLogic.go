package logic

import (
	"context"
	"strconv"
	"strings"

	"internalwallet/common/middleware"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTransactionRecordsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListBalanceRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTransactionRecordsLogic {
	return &ListTransactionRecordsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func NewListTransactionRecordsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTransactionRecordsLogic {
	return NewListBalanceRecordsLogic(ctx, svcCtx)
}

func (l *ListTransactionRecordsLogic) ListTransactionRecords(in *pb.ListTransactionRecordsReq) (*pb.ListTransactionRecordsResp, error) {
	uidStr := middleware.GetUserID(l.ctx)
	uid, _ := strconv.ParseInt(uidStr, 10, 64)

	if uid <= 0 || l.svcCtx.AccountingRpc == nil {
		return &pb.ListTransactionRecordsResp{Success: true, Total: 0, Items: []*pb.TransactionRecordItem{}}, nil
	}

	page := in.Page
	if page <= 0 {
		page = 1
	}
	size := in.PageSize
	if size <= 0 {
		size = 20
	}

	req := &pb.ListUserTransactionRecordsRequest{
		Page:     page,
		PageSize: size,
		UserId:   uid,
	}
	if in != nil {
		if asset := strings.TrimSpace(in.Asset); asset != "" {
			req.AssetCode = asset
		}
		if t := strings.TrimSpace(in.Type); t != "" {
			switch strings.ToLower(t) {
			case "deposit", "in", "income", "1":
				req.TxType = 1
			case "withdraw", "out", "expense", "2", "transfer", "internal_transfer", "internal":
				// 内部转账归入提现类型
				req.TxType = 2
			}
		}
	}

	accResp, err := l.svcCtx.AccountingRpc.ListUserTransactionRecords(l.ctx, req)
	if err != nil || accResp == nil || !accResp.Success {
		return &pb.ListTransactionRecordsResp{Success: true, Total: 0, Items: []*pb.TransactionRecordItem{}}, nil
	}

	// 收集唯一的 asset_code 和 chain_code，用于批量查询（单次遍历）
	assetSet := make(map[string]struct{})
	chainSet := make(map[string]struct{})
	for _, r := range accResp.GetItems() {
		if r == nil {
			continue
		}
		if code := strings.TrimSpace(r.AssetCode); code != "" {
			assetSet[strings.ToUpper(code)] = struct{}{}
		}
		if code := strings.TrimSpace(r.ChainCode); code != "" {
			chainSet[strings.ToUpper(code)] = struct{}{}
		}
	}

	// 批量加载资产图标
	assetIcons := make(map[string]string, len(assetSet))
	for assetCode := range assetSet {
		assetIcons[assetCode] = GetAssetIconURL(l.ctx, l.svcCtx, assetCode)
	}

	// 批量加载链显示名称
	chainInfoCache := make(map[string]string, len(chainSet))
	for chainCode := range chainSet {
		chain := GetChainInfo(l.ctx, l.svcCtx, chainCode)
		displayName := MapChainDisplayName(chainCode)
		if chain != nil && strings.TrimSpace(chain.Network) != "" {
			displayName = strings.TrimSpace(chain.Network)
		}
		chainInfoCache[chainCode] = displayName
	}

	items := make([]*pb.TransactionRecordItem, 0, len(accResp.GetItems()))
	for _, r := range accResp.GetItems() {
		if r == nil {
			continue
		}
		asset := strings.ToUpper(strings.TrimSpace(r.AssetCode))
		chainCode := strings.ToUpper(strings.TrimSpace(r.ChainCode))
		isDeposit := r.TxType == 1

		// Get chain display name
		chainName := chainCode
		if name, ok := chainInfoCache[chainCode]; ok {
			chainName = name
		}
		// 内部转账（无链）显示为"内部转账"
		if chainCode == "" {
			chainName = "内部转账"
		}

		// Get asset icon
		iconURL := ""
		if url, ok := assetIcons[asset]; ok {
			iconURL = url
		}

		items = append(items, &pb.TransactionRecordItem{
			Id:            strconv.FormatInt(r.Id, 10),
			Type:          pb.TransactionType(r.TxType),
			Asset:         asset,
			Amount:        strings.TrimSpace(r.AmountDecimal),
			Timestamp:     r.Timestamp,
			Status:        strings.TrimSpace(r.Status),
			Memo:          strings.TrimSpace(r.Memo),
			IconUrl:       iconURL,
			AmountDisplay: FormatAmountDisplay(r.AmountDecimal, asset, isDeposit),
			StatusText:    MapStatusText(r.Status),
			Chain:         chainCode,
			ChainName:     chainName,
		})
	}

	return &pb.ListTransactionRecordsResp{Success: true, Total: int32(accResp.Total), Items: items}, nil
}
