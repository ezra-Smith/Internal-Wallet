package logic

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
)

// Status mappings for UI display
var statusTextMap = map[string]string{
	"pending":    "处理中",
	"processing": "处理中",
	"confirmed":  "已确认",
	"success":    "已完成",
	"completed":  "已完成",
	"failed":     "交易失败",
	"rejected":   "已拒绝",
	"cancelled":  "已取消",
}

// Chain name mappings (chain code -> display name)
var chainDisplayNameMap = map[string]string{
	"TRON":    "TRC20",
	"ETH":     "ERC20",
	"BSC":     "BEP20",
	"POLYGON": "Polygon",
	"BTC":     "Bitcoin",
}

// Default explorer URL templates
var chainExplorerTemplates = map[string]string{
	"TRON":    "https://tronscan.org/#/transaction/%s",
	"ETH":     "https://etherscan.io/tx/%s",
	"BSC":     "https://bscscan.com/tx/%s",
	"POLYGON": "https://polygonscan.com/tx/%s",
	"BTC":     "https://mempool.space/tx/%s",
}

// Default confirmations required per chain
var chainConfirmationsMap = map[string]int32{
	"TRON":    1,
	"ETH":     12,
	"BSC":     3,
	"POLYGON": 128,
	"BTC":     3,
}

// MapStatusText converts internal status to UI-friendly text
func MapStatusText(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if text, ok := statusTextMap[status]; ok {
		return text
	}
	return status
}

// MapChainDisplayName converts chain code to display name
func MapChainDisplayName(chainCode string) string {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if name, ok := chainDisplayNameMap[chainCode]; ok {
		return name
	}
	return chainCode
}

// BuildExplorerURL builds the blockchain explorer URL for a transaction
func BuildExplorerURL(chainCode, txHash string) string {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return ""
	}
	if template, ok := chainExplorerTemplates[chainCode]; ok {
		return fmt.Sprintf(template, txHash)
	}
	return ""
}

// GetRequiredConfirmations returns the number of confirmations required for a chain
func GetRequiredConfirmations(chainCode string) int32 {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if confs, ok := chainConfirmationsMap[chainCode]; ok {
		return confs
	}
	return 12 // default
}

// FormatAmountDisplay formats amount for display (e.g., "+123.00 USDT" or "-50.00 BTC")
func FormatAmountDisplay(amount, asset string, isDeposit bool) string {
	amount = strings.TrimSpace(amount)
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if amount == "" {
		amount = "0"
	}
	prefix := "+"
	if !isDeposit {
		prefix = "-"
	}
	return fmt.Sprintf("%s%s %s", prefix, amount, asset)
}

// RawAmountToDecimalDisplay 将链上最小单位金额按资产精度转为展示用小数字符串（如 raw=990000, precision=6 => "0.99"）
func RawAmountToDecimalDisplay(rawStr string, precision int) string {
	rawStr = strings.TrimSpace(rawStr)
	if rawStr == "" || precision < 0 {
		return ""
	}
	d, err := decimal.NewFromString(rawStr)
	if err != nil || d.Sign() < 0 {
		return ""
	}
	divisor := decimal.NewFromInt(1).Shift(int32(precision))
	d = d.Div(divisor)
	s := trimTrailingZeros(d.String())
	if s == "" {
		return "0"
	}
	return s
}

var trailingZerosRegex = regexp.MustCompile(`\.?0+$`)

func trimTrailingZeros(s string) string {
	return trailingZerosRegex.ReplaceAllString(s, "")
}

var nonNegativeIntegerRegex = regexp.MustCompile(`^[0-9]+$`)

// ResolveWeb3DisplayAmount 统一计算 Web3 交易“展示用”金额。
//
// 背景：
// - 正常情况下：Amount 是已按 decimals 格式化的人类可读值；AmountRaw 是链上最小单位（整数）。
// - 但在极端/异常链路里（如 confirm processor 的 msg.Value 缺失时回退到 TokenAmount），
//   Amount 可能被写入最小单位整数，且与 AmountRaw 字符串相等，若仅靠相等判断会跳过转换并把 raw 展示给用户。
//
// 策略：
// - 只要 AmountRaw 看起来像“最小单位整数”且 decimals 可用，就优先用 AmountRaw + decimals 计算展示值。
// - decimals 优先用 tokenDecimals（链上精度），否则 fallback 到资产 precision。
// - 如果 AmountRaw 不是纯数字（例如已是小数），则回退使用 Amount。
func ResolveWeb3DisplayAmount(amount string, amountRaw *string, tokenDecimals *int32, assetPrecision int32) string {
	amount = strings.TrimSpace(amount)

	raw := ""
	if amountRaw != nil {
		raw = strings.TrimSpace(*amountRaw)
	}

	// 没有 raw 时，直接使用 amount
	if raw == "" || raw == "0" {
		if amount == "" {
			return "0"
		}
		return amount
	}

	// 选择 decimals：token_decimals > asset precision
	decimals := 0
	if tokenDecimals != nil && *tokenDecimals > 0 {
		decimals = int(*tokenDecimals)
	} else if assetPrecision > 0 {
		decimals = int(assetPrecision)
	}

	// 无有效精度，无法做 raw->display 转换
	if decimals <= 0 {
		if amount == "" {
			return raw
		}
		return amount
	}

	// 只有 raw 为“非负整数”时才认为是最小单位
	if !nonNegativeIntegerRegex.MatchString(raw) {
		if amount == "" {
			return raw
		}
		return amount
	}

	if displayed := RawAmountToDecimalDisplay(raw, decimals); displayed != "" {
		return displayed
	}

	// fallback
	if amount == "" {
		return raw
	}
	return amount
}

// GetAssetIconURL fetches the icon URL for an asset from Accounting service
func GetAssetIconURL(ctx context.Context, svcCtx *svc.ServiceContext, assetCode string) string {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" || svcCtx == nil || svcCtx.AccountingRpc == nil {
		return ""
	}
	resp, err := svcCtx.AccountingRpc.GetAsset(ctx, &pb.GetAssetRequest{Code: assetCode})
	if err != nil || resp == nil || !resp.Success || resp.Item == nil {
		return ""
	}
	return strings.TrimSpace(resp.Item.IconUrl)
}

// GetChainInfo fetches chain information from repository
func GetChainInfo(ctx context.Context, svcCtx *svc.ServiceContext, chainCode string) *model.ChainModel {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" || svcCtx == nil || svcCtx.ChainRepository == nil {
		return nil
	}
	chain, err := svcCtx.ChainRepository.FindByName(ctx, chainCode)
	if err != nil {
		return nil
	}
	return chain
}

// BuildExplorerURLFromChain builds explorer URL using chain info from database
func BuildExplorerURLFromChain(chain *model.ChainModel, txHash string) string {
	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		return ""
	}
	// Try using chain's explorer_url from database first
	if chain != nil && strings.TrimSpace(chain.ExplorerUrl) != "" {
		explorerBase := strings.TrimSpace(chain.ExplorerUrl)
		// Handle different URL patterns
		if strings.Contains(explorerBase, "%s") {
			return fmt.Sprintf(explorerBase, txHash)
		}
		// Append /tx/{hash} if base URL doesn't have format
		return strings.TrimSuffix(explorerBase, "/") + "/tx/" + txHash
	}
	// Fallback to hardcoded templates
	if chain != nil {
		return BuildExplorerURL(chain.Name, txHash)
	}
	return ""
}

// GetUserDepositAddress fetches user's deposit address for a chain
func GetUserDepositAddress(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, chainCode string) string {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" || svcCtx == nil || svcCtx.WalletDepositAddressRepository == nil {
		return ""
	}
	addr, err := svcCtx.WalletDepositAddressRepository.FindActiveByUserAndChain(ctx, userID, chainCode)
	if err != nil || addr == nil {
		return ""
	}
	return strings.TrimSpace(addr.Address)
}
