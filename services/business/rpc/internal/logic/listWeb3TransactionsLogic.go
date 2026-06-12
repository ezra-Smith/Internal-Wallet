package logic

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/repository"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListWeb3TransactionsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListWeb3TransactionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListWeb3TransactionsLogic {
	return &ListWeb3TransactionsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ListWeb3Transactions 获取 Web3 交易记录列表（公开查询，无需认证）
func (l *ListWeb3TransactionsLogic) ListWeb3Transactions(in *pb.ListWeb3TransactionsReq) (*pb.ListWeb3TransactionsResp, error) {
	// 验证 address
	address := strings.TrimSpace(in.Address)
	if address == "" {
		return nil, errx.Web3AddressRequired()
	}

	// 构建筛选条件
	filter := repository.Web3BalanceChangeListFilter{
		ChainCode: strings.ToUpper(strings.TrimSpace(in.ChainCode)), // 链筛选（如 ETH/BSC/TRON）
		AssetCode: strings.TrimSpace(in.AssetCode),
		TxType:    strings.ToLower(strings.TrimSpace(in.TxType)),
		SortBy:    "block_time",
		SortOrder: "desc",
	}

	// 时间筛选
	if in.StartTime > 0 {
		startTime := time.Unix(in.StartTime, 0)
		filter.FromTime = &startTime
	}
	if in.EndTime > 0 {
		endTime := time.Unix(in.EndTime, 0)
		filter.ToTime = &endTime
	}

	// 查询交易记录（根据地址公开查询）
	transactions, total, err := l.svcCtx.Web3BalanceChangeRepository.ListByAddress(
		l.ctx,
		address,
		in.Page,
		in.PageSize,
		filter,
	)
	if err != nil {
		l.Errorf("查询交易记录失败: address=%s, error=%v", address, err)
		return nil, errx.Web3AddressQueryFailed()
	}

	// 收集需要精度转换的 asset_code，批量查 Accounting 避免重复 RPC
	precisionMap := l.getAssetPrecisionMap(transactions)

	// 转换为响应格式
	items := make([]*pb.Web3TransactionItem, 0, len(transactions))
	for _, tx := range transactions {
		item := l.convertToTransactionItem(tx, precisionMap)
		items = append(items, item)
	}

	l.Infof("查询 Web3 交易记录成功: address=%s, total=%d", address, total)

	return &pb.ListWeb3TransactionsResp{
		Success: true,
		Message: "ok",
		Items:   items,
		Total:   total,
	}, nil
}

// getAssetPrecisionMap 对有 AmountRaw 的交易按 asset_code 查 Accounting 精度，返回 assetCode -> precision
func (l *ListWeb3TransactionsLogic) getAssetPrecisionMap(transactions []*model.Web3BalanceChangeModel) map[string]int32 {
	needPrecision := make(map[string]struct{})
	for _, tx := range transactions {
		if tx.AmountRaw != nil && strings.TrimSpace(*tx.AmountRaw) != "" && strings.TrimSpace(*tx.AmountRaw) != "0" {
			needPrecision[tx.AssetCode] = struct{}{}
		}
	}
	if len(needPrecision) == 0 || l.svcCtx.AccountingRpc == nil {
		return nil
	}
	out := make(map[string]int32, len(needPrecision))
	for code := range needPrecision {
		assetResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{Code: code})
		if err == nil && assetResp != nil && assetResp.Success && assetResp.Item != nil && assetResp.Item.Precision > 0 {
			out[code] = assetResp.Item.Precision
		}
	}
	return out
}

// convertToTransactionItem 转换交易记录为返回格式
func (l *ListWeb3TransactionsLogic) convertToTransactionItem(tx *model.Web3BalanceChangeModel, precisionMap map[string]int32) *pb.Web3TransactionItem {
	// 获取资产图标
	iconURL := l.getAssetIconURL(tx.AssetCode)

	// 金额展示：优先使用 AmountRaw + (token_decimals/资产精度) 计算，避免 Amount 在极端情况下被写入 raw 值导致误展示。
	var assetPrecision int32
	if precisionMap != nil {
		assetPrecision = precisionMap[tx.AssetCode]
	}
	amountStr := ResolveWeb3DisplayAmount(tx.Amount, tx.AmountRaw, tx.TokenDecimals, assetPrecision)
	amountDisplay := formatAmountDisplay(tx.Direction, amountStr, tx.AssetCode)

	// 获取交易类型文案（结合交易状态生成更准确的文案）
	txTypeText := getTxTypeText(tx.TxType, tx.Status)

	// 获取状态文案（结合交易类型生成更准确的文案）
	statusText := getStatusText(tx.Status, tx.TxType)

	return &pb.Web3TransactionItem{
		TxHash:        tx.TxHash,
		TxType:        tx.TxType,
		TxTypeText:    txTypeText,
		Direction:     tx.Direction,
		AssetCode:     tx.AssetCode,
		Amount:        amountStr,
		AmountDisplay: amountDisplay,
		AmountUsd:     "",
		FromAddress:   derefString(tx.FromAddress),
		ToAddress:     derefString(tx.ToAddress),
		Status:        tx.Status,
		StatusText:    statusText,
		BlockTime:     getTimestamp(tx.BlockTime),
		Network:       tx.ChainCode,
		IconUrl:       iconURL,
		CreatedAt:     tx.CreatedAt.Unix(), // 交易创建时间戳（Unix秒）
		Id:            tx.ID,
		EventIndex:    int32(tx.EventIndex),
		Fee:           derefString(tx.Fee),
		FeeAsset:      derefString(tx.FeeAsset),
	}
}

// getAssetIconURL 获取资产图标 URL（复用 getWeb3AssetOverviewLogic 的逻辑）
func (l *ListWeb3TransactionsLogic) getAssetIconURL(assetCode string) string {
	if l.svcCtx.AccountingRpc == nil {
		return ""
	}

	assetResp, err := l.svcCtx.AccountingRpc.GetAsset(l.ctx, &pb.GetAssetRequest{
		Code: assetCode,
	})
	if err != nil || !assetResp.Success || assetResp.Item == nil {
		return ""
	}

	return assetResp.Item.IconUrl
}

// formatAmountDisplay 格式化显示金额
func formatAmountDisplay(direction, amount, assetCode string) string {
	if direction == model.Web3TxDirectionIn {
		return fmt.Sprintf("+%s %s", amount, assetCode)
	}
	return fmt.Sprintf("-%s %s", amount, assetCode)
}

// getStatusText 获取状态文案（结合交易类型生成更准确的文案）
func getStatusText(status, txType string) string {
	switch status {
	case model.Web3TxStatusPending:
		// 根据交易类型显示更准确的状态
		switch txType {
		case model.Web3TxTypeSend:
			return "发送中"
		case model.Web3TxTypeReceive:
			return "接收中"
		case model.Web3TxTypeSwap:
			return "兑换中"
		case model.Web3TxTypeContractCall:
			return "执行中"
		case model.Web3TxTypeApprove:
			return "授权中"
		default:
			return "交易中"
		}
	case model.Web3TxStatusConfirmed:
		// 已确认的交易统一显示"已完成"
		return "已完成"
	case model.Web3TxStatusFailed:
		// 根据交易类型显示更准确的失败状态
		switch txType {
		case model.Web3TxTypeSend:
			return "发送失败"
		case model.Web3TxTypeReceive:
			return "接收失败"
		case model.Web3TxTypeSwap:
			return "兑换失败"
		case model.Web3TxTypeContractCall:
			return "执行失败"
		case model.Web3TxTypeApprove:
			return "授权失败"
		default:
			return "失败"
		}
	default:
		return "未知"
	}
}

// getTxTypeText 获取交易类型文案（结合交易状态生成更准确的文案）
func getTxTypeText(txType, status string) string {
	switch status {
	case model.Web3TxStatusPending:
		// 根据交易类型显示进行中的状态
		switch txType {
		case model.Web3TxTypeSend:
			return "发送中"
		case model.Web3TxTypeReceive:
			return "接收中"
		case model.Web3TxTypeSwap:
			return "兑换中"
		case model.Web3TxTypeContractCall:
			return "执行中"
		case model.Web3TxTypeApprove:
			return "授权中"
		default:
			return "交易中"
		}
	case model.Web3TxStatusFailed:
		// 根据交易类型显示失败状态
		switch txType {
		case model.Web3TxTypeSend:
			return "发送失败"
		case model.Web3TxTypeReceive:
			return "接收失败"
		case model.Web3TxTypeSwap:
			return "兑换失败"
		case model.Web3TxTypeContractCall:
			return "执行失败"
		case model.Web3TxTypeApprove:
			return "授权失败"
		default:
			return "交易失败"
		}
	case model.Web3TxStatusConfirmed:
		// 已确认的交易显示正常类型文案
		switch txType {
		case model.Web3TxTypeReceive:
			return "已接收"
		case model.Web3TxTypeSend:
			return "已发送"
		case model.Web3TxTypeSwap:
			return "代币兑换"
		case model.Web3TxTypeContractCall:
			return "合约调用"
		case model.Web3TxTypeApprove:
			return "授权"
		default:
			return txType
		}
	default:
		// 未知状态，返回基础类型文案
		switch txType {
		case model.Web3TxTypeReceive:
			return "已接收"
		case model.Web3TxTypeSend:
			return "已发送"
		case model.Web3TxTypeSwap:
			return "代币兑换"
		case model.Web3TxTypeContractCall:
			return "合约调用"
		case model.Web3TxTypeApprove:
			return "授权"
		default:
			return txType
		}
	}
}

// getTimestamp 转换时间为时间戳
func getTimestamp(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}

// stringPtr 返回字符串的指针（用于构建含 *string 字段的结构体）
func stringPtr(s string) *string {
	return &s
}

// derefString 安全解引用字符串指针，nil 时返回空字符串
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// derefInt64 安全解引用 int64 指针，nil 时返回 0
func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
