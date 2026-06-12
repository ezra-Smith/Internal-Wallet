package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"internalwallet/common/mq"
	commonutils "internalwallet/common/utils"
	"internalwallet/pkg/accounting"
	"internalwallet/pkg/notify"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/logic"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// TransactionConfirmProcessor 交易确认处理器
type TransactionConfirmProcessor struct {
	svcCtx *svc.ServiceContext
}

// NewTransactionConfirmProcessor 创建交易确认处理器
func NewTransactionConfirmProcessor(svcCtx *svc.ServiceContext) *TransactionConfirmProcessor {
	return &TransactionConfirmProcessor{
		svcCtx: svcCtx,
	}
}

// Process 处理交易确认消息
func (p *TransactionConfirmProcessor) Process(ctx context.Context, msg *mq.TransactionConfirmMessage) error {
	if msg == nil {
		return nil
	}

	switch msg.Source {
	case mq.AddressMonitorSourceDeposit:
		return p.processDepositConfirm(ctx, msg)
	case mq.AddressMonitorSourceWeb3:
		return p.processWeb3BalanceChangeForMonitoredAddress(ctx, msg)
	case mq.AddressMonitorSourceVault:
		logx.Infof("Skip vault tx confirm: tx_hash=%s chain=%s monitored=%s direction=%s", msg.TxHash, msg.Chain, msg.MonitoredAddress, msg.Direction)
		return nil
	case mq.AddressMonitorSourceCompany:
		return p.processCompanyConfirm(ctx, msg)
	case mq.AddressMonitorSourceManual, mq.AddressMonitorSourceUnknown, "":
		logx.Infof("Skip tx confirm: source=%s tx_hash=%s chain=%s monitored=%s", msg.Source, msg.TxHash, msg.Chain, msg.MonitoredAddress)
		return nil
	default:
		logx.Infof("Skip tx confirm: unsupported source=%s tx_hash=%s chain=%s monitored=%s", msg.Source, msg.TxHash, msg.Chain, msg.MonitoredAddress)
		return nil
	}
}

func normalizeTxDirection(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// processCompanyConfirm handles company-wallet confirmations that update system-ledger mirrors.
//
// Only accounts for: external inbound transfers to the system hot wallet address.
// Other company monitored addresses (non-hot) keep the legacy "skip" behavior.
func (p *TransactionConfirmProcessor) processCompanyConfirm(ctx context.Context, msg *mq.TransactionConfirmMessage) error {
	if msg == nil || p == nil || p.svcCtx == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Only care about inbound.
	if normalizeTxDirection(msg.Direction) != "in" {
		return nil
	}
	// Only care about EXTERNAL inflows. Internal rebalances between company wallets
	// require a different accounting op (transfer between system accounts), which we don't model yet.
	if msg.CounterpartyIsInternal {
		logx.Infof("Skip company inbound (internal counterparty): tx_hash=%s monitored=%s counterparty=%s",
			strings.TrimSpace(msg.TxHash), strings.TrimSpace(msg.MonitoredAddress), strings.TrimSpace(msg.CounterpartyAddress))
		return nil
	}

	chainCode := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(msg.Chain)))
	if chainCode == "" {
		return nil
	}
	if p.svcCtx.SignerRpc == nil || p.svcCtx.AccountingRpc == nil {
		// Can't fund system wallet without signer+accounting.
		return nil
	}

	// Only fund when the monitored address is the configured system hot wallet.
	hotAddr, err := p.getSystemHotWalletAddress(ctx, chainCode)
	if err != nil || hotAddr == "" {
		logx.WithContext(ctx).Errorf("Failed to resolve system hot wallet address: chain=%s err=%v", chainCode, err)
		return nil
	}
	monitored := strings.ToLower(strings.TrimSpace(msg.MonitoredAddress))
	if monitored == "" || monitored != strings.ToLower(strings.TrimSpace(hotAddr)) {
		// Not a hot wallet movement: keep legacy skip.
		return nil
	}

	assetCode := p.determineAssetCode(ctx, msg)
	if strings.TrimSpace(assetCode) == "" {
		// Fallbacks: for tokens, prefer symbol; for native, prefer chain code.
		if msg.IsTokenTransaction() {
			assetCode = strings.ToUpper(strings.TrimSpace(msg.TokenSymbol))
		} else {
			assetCode = chainCode
		}
	}
	assetCode = accounting.NormalizeAssetCode(assetCode)
	if assetCode == "" {
		return nil
	}

	// Normalize amount to Accounting asset precision. TokenAmount may be raw smallest-unit integer;
	// and some chains (e.g. BSC stablecoins) use on-chain decimals != asset precision.
	assetResp, err := p.svcCtx.AccountingRpc.GetAsset(ctx, &pb.GetAssetRequest{Code: assetCode})
	if err != nil {
		logx.WithContext(ctx).Errorf("Accounting GetAsset failed (skip funding): tx_hash=%s chain=%s asset=%s err=%v",
			strings.TrimSpace(msg.TxHash), chainCode, assetCode, err)
		return nil
	}
	if assetResp == nil || !assetResp.Success || assetResp.Item == nil || assetResp.Item.Status != 1 {
		logx.WithContext(ctx).Errorf("Asset not available (skip funding): tx_hash=%s chain=%s asset=%s",
			strings.TrimSpace(msg.TxHash), chainCode, assetCode)
		return nil
	}

	amountDec, err := normalizeDepositAmountDecimal(msg, assetResp.Item.Precision)
	if err != nil {
		logx.WithContext(ctx).Errorf("Normalize amount failed (skip funding): tx_hash=%s chain=%s asset=%s err=%v",
			strings.TrimSpace(msg.TxHash), chainCode, assetCode, err)
		return nil
	}
	amountDec = strings.TrimSpace(amountDec)
	if amountDec == "" || amountDec == "0" {
		return nil
	}

	// Idempotency key dedupes by tx+address+direction+log_index+token so duplicate deliveries are safe.
	idemKey := systemWalletFundIdempotencyKey(msg)
	bizRef := fmt.Sprintf("tx:%s:%d", strings.TrimSpace(msg.TxHash), msg.LogIndex)

	resp, rpcErr := p.svcCtx.AccountingRpc.FundSystemWallet(ctx, &pb.FundSystemWalletRequest{
		IdempotencyKey: idemKey,
		BizRef:         bizRef,
		ChainCode:      chainCode,
		AssetCode:      assetCode,
		AmountDecimal:  amountDec,
	})
	if rpcErr != nil {
		logx.WithContext(ctx).Errorf("FundSystemWallet rpc failed: tx_hash=%s chain=%s asset=%s amount=%s err=%v",
			strings.TrimSpace(msg.TxHash), chainCode, assetCode, amountDec, rpcErr)
		return nil
	}
	if resp == nil || !resp.Success {
		msgText := ""
		if resp != nil {
			msgText = strings.TrimSpace(resp.Message)
		}
		logx.WithContext(ctx).Errorf("FundSystemWallet failed: tx_hash=%s chain=%s asset=%s amount=%s success=%v message=%s",
			strings.TrimSpace(msg.TxHash), chainCode, assetCode, amountDec, resp != nil && resp.Success, msgText)
		return nil
	}

	logx.WithContext(ctx).Infof("✅ Funded system hot wallet from external inbound tx: tx_hash=%s chain=%s asset=%s amount=%s tx_id=%d duplicate=%v",
		strings.TrimSpace(msg.TxHash), chainCode, assetCode, amountDec, resp.TxId, resp.Duplicate)
	return nil
}

func (p *TransactionConfirmProcessor) getSystemHotWalletAddress(ctx context.Context, chainCode string) (string, error) {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return "", fmt.Errorf("empty chain code")
	}
	resp, err := p.svcCtx.SignerRpc.GetCompanyWallet(ctx, &pb.GetCompanyWalletRequest{
		Chain:       chainCode,
		AddressType: "hot_primary",
		Temperature: 1,
	})
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("empty signer response")
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("signer error: code=%d message=%s", resp.Code, strings.TrimSpace(resp.Message))
	}
	if resp.Wallet == nil || strings.TrimSpace(resp.Wallet.Address) == "" {
		return "", fmt.Errorf("empty hot wallet address")
	}
	if resp.Wallet.Status != 1 {
		return "", fmt.Errorf("hot wallet disabled")
	}
	return strings.TrimSpace(resp.Wallet.Address), nil
}

func systemWalletFundIdempotencyKey(msg *mq.TransactionConfirmMessage) string {
	// Dedupe across source fan-out: exclude msg.Source.
	// Include log_index to dedupe multi-transfer events within one tx.
	txHash := ""
	monitored := ""
	dir := ""
	tokenAddr := ""
	tokenAmt := ""
	logIndex := int32(0)
	if msg != nil {
		txHash = strings.ToLower(strings.TrimSpace(msg.TxHash))
		monitored = strings.ToLower(strings.TrimSpace(msg.MonitoredAddress))
		dir = strings.ToLower(strings.TrimSpace(msg.Direction))
		tokenAddr = strings.ToLower(strings.TrimSpace(msg.TokenAddress))
		tokenAmt = strings.TrimSpace(msg.TokenAmount)
		logIndex = msg.LogIndex
	}
	payload := fmt.Sprintf("%s|%s|%s|%d|%s|%s", txHash, monitored, dir, logIndex, tokenAddr, tokenAmt)
	sum := sha256.Sum256([]byte(payload))
	return "sys_wallet:fund:" + hex.EncodeToString(sum[:])
}

func (p *TransactionConfirmProcessor) processDepositConfirm(ctx context.Context, msg *mq.TransactionConfirmMessage) error {
	if msg == nil {
		return nil
	}

	if normalizeTxDirection(msg.Direction) != "in" {
		// Deposit source emits both directions; Business only confirms inbound deposits.
		return nil
	}
	if msg.CounterpartyIsInternal {
		// Internal->internal transfers are not user deposits.
		return nil
	}

	web2UserAddress, err := p.findWeb2UserAddress(ctx, msg.MonitoredAddress, msg.Chain)
	if err != nil {
		return err
	}
	if web2UserAddress == nil {
		logx.Infof("Deposit address not found, skipping: tx_hash=%s chain=%s monitored=%s", msg.TxHash, msg.Chain, msg.MonitoredAddress)
		return nil
	}

	return p.processWeb2Transaction(ctx, web2UserAddress, msg)
}

func (p *TransactionConfirmProcessor) processWeb3BalanceChangeForMonitoredAddress(ctx context.Context, msg *mq.TransactionConfirmMessage) error {
	if msg == nil {
		return nil
	}
	if p == nil || p.svcCtx == nil || p.svcCtx.Web3TransactionRepository == nil {
		return fmt.Errorf("web3 transaction repository not configured")
	}

	chainID := msg.ChainId
	if chainID <= 0 {
		chainID = p.chainStringToChainID(msg.Chain)
	}
	addr, err := p.findWeb3UserAddress(ctx, msg.MonitoredAddress, msg.Chain, chainID)
	if err != nil {
		return err
	}
	if addr == nil {
		logx.Infof("Web3 address not found, skipping: tx_hash=%s chain=%s chain_id=%d monitored=%s", msg.TxHash, msg.Chain, chainID, msg.MonitoredAddress)
		return nil
	}

	// Use the scanned chain_id for downstream records (balance change + balance sync) to keep metadata consistent
	// with the chain chainsync is currently monitoring, even if the address was originally stored with a testnet id.
	effectiveAddr := *addr
	if chainID > 0 {
		effectiveAddr.ChainID = chainID
	}

	assetCode := p.determineAssetCode(ctx, msg)
	if assetCode == "" {
		logx.Errorf("Cannot determine asset code for web3 change, skipping: tx_hash=%s chain=%s token_address=%s",
			msg.TxHash, msg.Chain, msg.TokenAddress)
		return fmt.Errorf("cannot determine asset code for transaction")
	}

	amount := p.determineDisplayAmount(msg) // 展示用金额（十进制字符串）
	blockTime := p.convertBlockTime(msg.BlockTimestamp)
	chainCode := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(msg.Chain)))
	if chainCode == "" {
		return fmt.Errorf("invalid chain: %s", strings.TrimSpace(msg.Chain))
	}

	direction := normalizeTxDirection(msg.Direction)
	switch direction {
	case "in":
		direction = model.Web3TxDirectionIn
	case "out":
		direction = model.Web3TxDirectionOut
	default:
		direction = p.determineDirection(msg, effectiveAddr.Address)
	}

	if err := p.upsertWeb3BalanceChangeForAddress(ctx, &effectiveAddr, msg, chainCode, blockTime, assetCode, amount, direction); err != nil {
		return err
	}

	// Token 交易的 gas 消耗只影响发送方
	if msg.IsTokenTransaction() && direction == model.Web3TxDirectionOut {
		_ = p.syncWeb3NativeBalanceFromChain(ctx, &effectiveAddr, msg)
	}

	// Also write a "tx-level" record for legacy consumers that still read `web3_transactions`.
	// Note: current `web3_transactions` schema is tx_hash-unique, so we keep sender side (out) as primary.
	if direction == model.Web3TxDirectionOut {
		return p.upsertWeb3TransactionForAddress(ctx, &effectiveAddr, msg, chainCode, blockTime, assetCode, amount, direction)
	}

	// 入账交易：检查是否已有记录
	existing, err := p.svcCtx.Web3TransactionRepository.FindByTxHash(ctx, strings.TrimSpace(msg.TxHash))
	//error=find web3 tx by hash failed: record not found  需要当错误不是gorm.ErrRecordNotFound时才是真的错误
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find web3 tx by hash failed: %w", err)
	}
	if existing != nil && strings.EqualFold(strings.TrimSpace(existing.Direction), model.Web3TxDirectionOut) {
		return nil
	}

	// 没有记录，写入入账记录（纯收款交易）
	logx.Infof("Writing web3_transaction (IN): tx_hash=%s user_address=%s (new record)",
		msg.TxHash, effectiveAddr.Address)
	return p.upsertWeb3TransactionForAddress(ctx, &effectiveAddr, msg, chainCode, blockTime, assetCode, amount, direction)
}

// processWeb3BalanceChanges 处理 Web3 地址的“资产变动事件”记录
// - 同一 tx_hash 会有多条事件（swap、多 token transfer、internal transfers）
// - 同一事件可能同时影响 from/to 两个监控地址（内部转账）
func (p *TransactionConfirmProcessor) processWeb3BalanceChanges(ctx context.Context, fromAddr, toAddr *model.Web3UserAddressModel, msg *mq.TransactionConfirmMessage) error {
	if p.svcCtx == nil || p.svcCtx.Web3BalanceChangeRepository == nil {
		return fmt.Errorf("web3 balance change repository not configured")
	}
	if msg == nil {
		return nil
	}

	assetCode := p.determineAssetCode(ctx, msg)
	if assetCode == "" {
		logx.Errorf("Cannot determine asset code for web3 change, skipping: tx_hash=%s chain=%s token_address=%s",
			msg.TxHash, msg.Chain, msg.TokenAddress)
		return fmt.Errorf("cannot determine asset code for transaction")
	}

	amount := p.determineDisplayAmount(msg) // 展示用金额（十进制字符串）
	blockTime := p.convertBlockTime(msg.BlockTimestamp)
	chainCode := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(msg.Chain)))

	// 同一地址既是 from 又是 to（自转账）场景：只生成一条 out（避免重复）。
	sameAddr := fromAddr != nil && toAddr != nil && strings.EqualFold(strings.TrimSpace(fromAddr.Address), strings.TrimSpace(toAddr.Address))

	if fromAddr != nil {
		if err := p.upsertWeb3BalanceChangeForAddress(ctx, fromAddr, msg, chainCode, blockTime, assetCode, amount, model.Web3TxDirectionOut); err != nil {
			return err
		}
		// Token 交易的 gas 消耗只影响发送方
		if msg.IsTokenTransaction() {
			_ = p.syncWeb3NativeBalanceFromChain(ctx, fromAddr, msg)
		}
	}

	if toAddr != nil && !sameAddr {
		if err := p.upsertWeb3BalanceChangeForAddress(ctx, toAddr, msg, chainCode, blockTime, assetCode, amount, model.Web3TxDirectionIn); err != nil {
			return err
		}
	}

	// Also write a "tx-level" record for legacy consumers that still read `web3_transactions`.
	// Note: current `web3_transactions` schema is tx_hash-unique, so we can only keep one row per tx.
	// Prefer the sender side when available (out), otherwise receiver side (in).
	primaryAddr := fromAddr
	primaryDir := model.Web3TxDirectionOut
	if primaryAddr == nil {
		primaryAddr = toAddr
		primaryDir = model.Web3TxDirectionIn
	}
	if primaryAddr != nil {
		if err := p.upsertWeb3TransactionForAddress(ctx, primaryAddr, msg, chainCode, blockTime, assetCode, amount, primaryDir); err != nil {
			return err
		}
	}

	return nil
}

// processWeb2Transaction 处理 Web2 用户交易
func (p *TransactionConfirmProcessor) processWeb2Transaction(ctx context.Context, addr *model.WalletDepositAddressModel, msg *mq.TransactionConfirmMessage) error {
	if addr == nil || msg == nil {
		return nil
	}
	if p.svcCtx == nil || p.svcCtx.DB == nil {
		return fmt.Errorf("business gorm db not configured")
	}
	if p.svcCtx.AccountingRpc == nil {
		return fmt.Errorf("accounting rpc not configured")
	}

	chainCode := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(msg.Chain)))
	if chainCode == "" {
		return fmt.Errorf("invalid chain: %s", strings.TrimSpace(msg.Chain))
	}
	if strings.ToUpper(strings.TrimSpace(addr.ChainCode)) != chainCode {
		logx.Infof("Skip Web2 tx due to chain mismatch: tx_hash=%s addr_chain=%s msg_chain=%s", msg.TxHash, addr.ChainCode, chainCode)
		return nil
	}

	// Web2 充值地址验证
	// 对于代币交易（ERC20/TRC20）：
	//   - msg.TokenAddress 是代币合约地址
	//   - msg.ToAddress 是接收方地址（不是合约地址）
	//   - msg.MonitoredAddress 应该是实际接收者（chainsync 从 Transfer 事件解析）
	// 但是，如果 chainsync 设置错误（monitored_address 是发送方），则检查 to_address
	// 对于主币交易：
	//   - msg.ToAddress 是接收方地址
	//   - msg.MonitoredAddress 应该是接收方地址
	matched := sameChainAddress(addr.Address, msg.MonitoredAddress)
	if !matched && msg.IsTokenTransaction() {
		// 对于代币交易，如果 monitored_address 不匹配，尝试检查 to_address
		// 因为 chainsync 可能将 monitored_address 设置为发送方地址
		// 而 to_address 是实际的接收方地址
		matched = sameChainAddress(addr.Address, msg.ToAddress)
	}
	if !matched {
		logx.Infof("Skip Web2 tx: address mismatch: tx_hash=%s monitored=%s to=%s deposit_addr=%s",
			msg.TxHash, msg.MonitoredAddress, msg.ToAddress, addr.Address)
		return nil
	}

	assetCode, err := p.resolveDepositAssetCode(ctx, chainCode, msg)
	if err != nil {
		return err
	}
	if assetCode == "" {
		logx.Infof("Skip Web2 tx: asset not resolved: tx_hash=%s chain=%s token=%t contract=%s symbol=%s", msg.TxHash, chainCode, msg.IsTokenTransaction(), msg.TokenAddress, msg.TokenSymbol)
		return nil
	}

	assetResp, err := p.svcCtx.AccountingRpc.GetAsset(ctx, &pb.GetAssetRequest{Code: assetCode})
	if err != nil {
		return fmt.Errorf("accounting GetAsset failed: %w", err)
	}
	if assetResp == nil || !assetResp.Success || assetResp.Item == nil || assetResp.Item.Status != 1 {
		return fmt.Errorf("asset not available: %s", assetCode)
	}

	amountDecimal, err := normalizeDepositAmountDecimal(msg, assetResp.Item.Precision)
	if err != nil {
		return err
	}

	idemKey, bizRef := depositConfirmIdemKeyAndBizRef(msg.TxHash)
	confirmResp, confirmErr := p.svcCtx.AccountingRpc.ConfirmDeposit(ctx, &pb.ConfirmDepositRequest{
		IdempotencyKey: idemKey,
		BizRef:         bizRef,
		UserId:         addr.UserID,
		AssetCode:      assetCode,
		ChainCode:      chainCode,
		AmountDecimal:  amountDecimal,
		TxHash:         strings.TrimSpace(msg.TxHash),
		FromAddress:    strings.TrimSpace(msg.FromAddress),
		ToAddress:      strings.TrimSpace(addr.Address),
		Memo: func() string {
			if addr.Memo == nil {
				return ""
			}
			return strings.TrimSpace(*addr.Memo)
		}(),
	})
	if confirmErr != nil {
		return fmt.Errorf("accounting ConfirmDeposit failed: %w", confirmErr)
	}
	if confirmResp == nil || !confirmResp.Success {
		msgText := ""
		if confirmResp != nil {
			msgText = strings.TrimSpace(confirmResp.Message)
		}
		if msgText == "" {
			msgText = "confirm deposit failed"
		}
		return fmt.Errorf("%s", msgText)
	}

	if err := p.upsertWalletDepositRecord(ctx, addr, msg, chainCode, assetCode, amountDecimal); err != nil {
		return err
	}

	// 更新充值地址余额
	// 注意：此操作失败不应影响主流程（充值已确认），仅记录错误
	if err := p.updateDepositAddressBalanceViaRPC(ctx, addr, msg, chainCode, assetCode, amountDecimal, assetResp.Item.Precision); err != nil {
		logx.Errorf("❌ Failed to update deposit address balance (non-critical): tx_hash=%s user_id=%d error=%v", msg.TxHash, addr.UserID, err)
		// 不返回错误，避免影响充值确认主流程
	}

	logx.Infof("✅ Confirmed Web2 deposit: tx_hash=%s user_id=%d amount=%s %s duplicate=%t", msg.TxHash, addr.UserID, amountDecimal, assetCode, confirmResp.Duplicate)

	// 异步发送充值成功邮件通知（不阻塞响应）
	if !confirmResp.Duplicate {
		// 仅在非重复确认时发送邮件，避免重复通知
		go p.sendDepositSuccessEmail(addr.UserID, assetCode, chainCode, amountDecimal, msg, addr)
	}

	return nil
}

// findWeb3UserAddress 根据地址和链查找 Web3 用户地址
func (p *TransactionConfirmProcessor) findWeb3UserAddress(ctx context.Context, address string, chain string, chainIDFromMsg int64) (*model.Web3UserAddressModel, error) {
	// 将 chain 字符串转换为 chainId（若消息已携带 chain_id，则优先使用消息值）
	chainId := chainIDFromMsg
	if chainId <= 0 {
		chainId = p.chainStringToChainID(chain)
	}

	// 查询 web3_user_addresses 表
	var addr model.Web3UserAddressModel
	address = strings.TrimSpace(address)
	addressIsEVM := strings.HasPrefix(strings.ToLower(address), "0x")

	// 1) Prefer strict match on (chain_id, address) to avoid mixing networks when data is clean.
	{
		q := p.svcCtx.Web3UserAddressRepository.GetDB().WithContext(ctx).
			Model(&model.Web3UserAddressModel{}).
			Where("chain_id = ? AND deleted_at IS NULL", chainId)
		if addressIsEVM {
			q = q.Where("LOWER(address) = ?", strings.ToLower(address))
		} else {
			q = q.Where("address = ?", address)
		}
		err := q.First(&addr).Error
		if err == nil {
			return &addr, nil
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, err
		}
	}

	// 2) Fallback: chainsync monitors by chain type (ETH/BSC/TRON). Some clients may store testnet chain_id
	// (e.g. 97/11155111) while chainsync is configured for mainnet (56/1). In that case, we still want to
	// generate balance changes as long as the address exists for the chain type.
	chainIDs := p.fallbackChainIDsForChain(chain, chainId)
	if len(chainIDs) == 0 {
		return nil, nil
	}

	q2 := p.svcCtx.Web3UserAddressRepository.GetDB().WithContext(ctx).
		Model(&model.Web3UserAddressModel{}).
		Where("chain_id IN ? AND deleted_at IS NULL", chainIDs).
		Order("is_primary DESC, id DESC")
	if addressIsEVM {
		q2 = q2.Where("LOWER(address) = ?", strings.ToLower(address))
	} else {
		q2 = q2.Where("address = ?", address)
	}
	err := q2.First(&addr).Error

	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &addr, nil
}

func (p *TransactionConfirmProcessor) fallbackChainIDsForChain(chain string, preferred int64) []int64 {
	code := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(chain)))
	var base []int64
	switch code {
	case "ETH":
		base = []int64{1, 11155111}
	case "BSC":
		base = []int64{56, 97}
	case "TRON":
		base = []int64{728126428}
	default:
		base = nil
	}
	if preferred > 0 {
		base = append([]int64{preferred}, base...)
	}
	// Deduplicate while preserving order.
	out := make([]int64, 0, len(base))
	seen := make(map[int64]struct{}, len(base))
	for _, id := range base {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// determineDirection 确定交易方向（in/out）
func (p *TransactionConfirmProcessor) determineDirection(msg *mq.TransactionConfirmMessage, userAddress string) string {
	// 标准化地址比较（不区分大小写）
	userAddrLower := strings.ToLower(userAddress)
	fromAddrLower := strings.ToLower(msg.FromAddress)
	toAddrLower := strings.ToLower(msg.ToAddress)

	// 如果用户地址是接收方，则为收入
	if toAddrLower == userAddrLower {
		return model.Web3TxDirectionIn
	}

	// 如果用户地址是发送方，则为支出
	if fromAddrLower == userAddrLower {
		return model.Web3TxDirectionOut
	}

	// 如果上述判断都不匹配，使用监控地址作为 fallback
	// 注意：对于代币交易，MonitoredAddress 可能与 userAddress 不同
	monitoredAddrLower := strings.ToLower(msg.MonitoredAddress)
	if monitoredAddrLower == toAddrLower {
		return model.Web3TxDirectionIn
	}
	return model.Web3TxDirectionOut
}

// determineTransactionType 确定交易类型
func (p *TransactionConfirmProcessor) determineTransactionType(msg *mq.TransactionConfirmMessage, direction string) string {
	// 如果是代币交易
	if msg.IsTokenTransaction() {
		// 可以根据合约地址判断是否是 swap 或 approve
		// 这里简化处理，默认为 receive/send
		if direction == model.Web3TxDirectionIn {
			return model.Web3TxTypeReceive
		}
		return model.Web3TxTypeSend
	}

	// 主币交易
	if direction == model.Web3TxDirectionIn {
		return model.Web3TxTypeReceive
	}
	return model.Web3TxTypeSend
}

// determineAssetCode 确定资产代码
func (p *TransactionConfirmProcessor) determineAssetCode(ctx context.Context, msg *mq.TransactionConfirmMessage) string {
	if msg.IsTokenTransaction() {
		// Prefer token symbol when it is meaningful; some chainsync paths emit "UNKNOWN".
		symbol := strings.ToUpper(strings.TrimSpace(msg.TokenSymbol))
		if symbol != "" && symbol != "UNKNOWN" {
			return symbol
		}
		// Fallback: resolve by chain + token contract address from currency_chain_settings.
		inferred := p.inferTokenSymbol(ctx, msg.Chain, msg.TokenAddress)
		if inferred == "" {
			logx.Infof("⚠️  Cannot determine asset code for token tx: chain=%s contract=%s symbol=%s", msg.Chain, msg.TokenAddress, msg.TokenSymbol)
		}
		return inferred
	}

	// 主币交易，根据链确定
	return p.getNativeTokenSymbol(msg.Chain)
}

// determineAmount 确定交易金额（标准化为最小单位的原始值）
func (p *TransactionConfirmProcessor) determineAmount(msg *mq.TransactionConfirmMessage) string {
	var rawAmount string
	if msg.IsTokenTransaction() {
		rawAmount = msg.TokenAmount
	} else {
		rawAmount = msg.Value
	}

	// 标准化金额：移除可能的单位后缀（如 "1.5 ETH" -> "1.5"）
	rawAmount = strings.TrimSpace(rawAmount)
	if rawAmount == "" {
		return "0"
	}

	// 如果是格式化值（如 "1.5 ETH"），提取数字部分
	if parts := strings.Fields(rawAmount); len(parts) >= 2 {
		rawAmount = parts[0]
	}

	// 如果包含小数点，说明已经是格式化的数量，需要转换为最小单位
	// 但这里我们保持原始格式，交给前端或其他地方处理
	// 确保是有效数字
	if _, err := decimal.NewFromString(rawAmount); err != nil {
		logx.Infof("⚠️  Invalid amount format: %s, using 0", rawAmount)
		return "0"
	}

	return rawAmount
}

// determineDisplayAmount 获取“展示用”的金额：
// - Token 交易优先使用 msg.Value（chainsync 已按 decimals 格式化）
// - raw 最小单位（TokenAmount）仅作为 amount_raw 存储
func (p *TransactionConfirmProcessor) determineDisplayAmount(msg *mq.TransactionConfirmMessage) string {
	if msg == nil {
		return "0"
	}

	amount := strings.TrimSpace(msg.Value)
	if amount == "" {
		// 兼容极端情况：Value 缺失时，优先尝试把 TokenAmount(最小单位) 按 TokenDecimals 转为人类可读值。
		// 否则直接回退 TokenAmount 会把 raw 值写入 Amount，导致后续展示/计价出错。
		tokenAmount := strings.TrimSpace(msg.TokenAmount)
		if msg.IsTokenTransaction() && tokenAmount != "" && msg.TokenDecimals > 0 {
			// 如果是格式化值（如 "1000000 USDT"），提取数字部分
			if parts := strings.Fields(tokenAmount); len(parts) >= 2 {
				tokenAmount = parts[0]
			}

			if d, err := decimal.NewFromString(tokenAmount); err == nil && d.Sign() >= 0 {
				divisor := decimal.NewFromInt(1).Shift(int32(msg.TokenDecimals))
				converted := d.Div(divisor).String()
				converted = strings.TrimSpace(converted)
				if converted != "" {
					amount = converted
				}
			}
		}

		// 最后兜底：仍然用 TokenAmount
		if amount == "" {
			amount = tokenAmount
		}
	}
	if amount == "" {
		return "0"
	}

	// 如果是格式化值（如 "1.5 ETH"），提取数字部分
	if parts := strings.Fields(amount); len(parts) >= 2 {
		amount = parts[0]
	}

	if _, err := decimal.NewFromString(amount); err != nil {
		logx.Infof("⚠️  Invalid display amount format: %s, using 0", amount)
		return "0"
	}
	return amount
}

// convertBlockTime 转换区块时间戳
func (p *TransactionConfirmProcessor) convertBlockTime(timestamp int64) *time.Time {
	if timestamp == 0 {
		return nil
	}
	t := time.Unix(timestamp, 0)
	return &t
}

func (p *TransactionConfirmProcessor) resolveWeb3TxType(ctx context.Context, txHash string, userAddress string, fallback string) string {
	if strings.TrimSpace(fallback) == "" {
		fallback = model.Web3TxTypeSend
	}
	if p == nil || p.svcCtx == nil || p.svcCtx.Web3BalanceChangeRepository == nil {
		return fallback
	}

	placeholder, err := p.svcCtx.Web3BalanceChangeRepository.FindBroadcastPlaceholder(ctx, txHash, userAddress)
	if err == nil && placeholder != nil {
		if s := strings.TrimSpace(placeholder.TxType); s != "" {
			return s
		}
	}
	return fallback
}

func (p *TransactionConfirmProcessor) upsertWeb3BalanceChangeForAddress(
	ctx context.Context,
	addr *model.Web3UserAddressModel,
	msg *mq.TransactionConfirmMessage,
	chainCode string,
	blockTime *time.Time,
	assetCode string,
	amount string,
	direction string,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if p == nil || p.svcCtx == nil || p.svcCtx.Web3BalanceChangeRepository == nil {
		return fmt.Errorf("web3 balance change repository not configured")
	}
	if addr == nil || msg == nil {
		return nil
	}

	txHash := strings.TrimSpace(msg.TxHash)
	if txHash == "" {
		return nil
	}
	userAddress := strings.TrimSpace(addr.Address)
	if userAddress == "" {
		return nil
	}

	// Link the confirmed record to the broadcast placeholder (event_index = -1),
	// so clients can still query detail by the old placeholder id after it is soft-deleted.
	var oldID *int64
	if placeholder, err := p.svcCtx.Web3BalanceChangeRepository.FindBroadcastPlaceholder(ctx, txHash, userAddress); err == nil && placeholder != nil {
		v := placeholder.ID
		oldID = &v
	}

	// tx_type：默认 receive/send；若用户侧广播时提供 swap 等类型，优先复用广播占位记录。
	fallbackTxType := p.determineTransactionType(msg, direction)
	txType := p.resolveWeb3TxType(ctx, txHash, userAddress, fallbackTxType)

	// amount_raw（最小单位）仅用于存档/调试
	var amountRaw *string
	if s := strings.TrimSpace(msg.TokenAmount); s != "" {
		amountRaw = &s
	}

	var tokenAddress *string
	var tokenDecimals *int32
	if msg.IsTokenTransaction() {
		if s := strings.TrimSpace(msg.TokenAddress); s != "" {
			tokenAddress = &s
		}
		if msg.TokenDecimals > 0 {
			d := int32(msg.TokenDecimals)
			tokenDecimals = &d
		}
	}

	var fromAddr *string
	if s := strings.TrimSpace(msg.FromAddress); s != "" {
		fromAddr = &s
	}
	var toAddr *string
	if s := strings.TrimSpace(msg.ToAddress); s != "" {
		toAddr = &s
	}

	var blockNumber *int64
	if msg.BlockNumber > 0 {
		bn := int64(msg.BlockNumber)
		blockNumber = &bn
	}

	var fee *string
	fee, feeAssetPtr := commonutils.FormatWeb3Fee(chainCode, msg.GasFee, msg.GasUsed, msg.GasPrice)

	raw := map[string]interface{}{
		"chain":                  msg.Chain,
		"transaction_type":       msg.TransactionType,
		"monitored_address":      msg.MonitoredAddress,
		"required_confirmations": msg.RequiredConfirmations,
		"log_index":              msg.LogIndex,
		"token_address":          msg.TokenAddress,
		"token_name":             msg.TokenName,
		"token_symbol":           msg.TokenSymbol,
		"token_decimals":         msg.TokenDecimals,
		"token_amount":           msg.TokenAmount,
		"value":                  msg.Value,
	}
	rawBytes, _ := json.Marshal(raw)

	change := &model.Web3BalanceChangeModel{
		OldID: oldID,

		Web3UserID:    addr.Web3UserID,
		UserAddress:   userAddress,
		ChainCode:     strings.ToUpper(strings.TrimSpace(chainCode)),
		ChainID:       addr.ChainID,
		TxHash:        txHash,
		EventIndex:    msg.LogIndex,
		BlockNumber:   blockNumber,
		BlockTime:     blockTime,
		TxType:        txType,
		Direction:     direction,
		AssetCode:     strings.ToUpper(strings.TrimSpace(assetCode)),
		Amount:        strings.TrimSpace(amount),
		AmountRaw:     amountRaw,
		TokenAddress:  tokenAddress,
		TokenDecimals: tokenDecimals,
		FromAddress:   fromAddr,
		ToAddress:     toAddr,
		Status:        model.Web3TxStatusConfirmed,
		Confirmations: msg.Confirmations,
		Fee:           fee,
		FeeAsset:      feeAssetPtr,
		RawData:       rawBytes,
	}

	if err := p.svcCtx.Web3BalanceChangeRepository.Upsert(ctx, change); err != nil {
		return fmt.Errorf("upsert web3 balance change failed: %w", err)
	}

	// 删除（软删）广播占位记录（避免 confirmed 后 list 出现重复占位）
	_ = p.svcCtx.Web3BalanceChangeRepository.SoftDeleteBroadcastPlaceholder(ctx, txHash, userAddress)

	// 同步余额（不影响主流程）
	if err := p.syncWeb3AddressBalanceFromChain(ctx, addr, msg); err != nil {
		logx.Errorf("❌ Failed to sync Web3 address balance from chain: address=%s chain=%s tx_hash=%s error=%v",
			addr.Address, msg.Chain, msg.TxHash, err)
	}

	return nil
}

func (p *TransactionConfirmProcessor) upsertWeb3TransactionForAddress(
	ctx context.Context,
	addr *model.Web3UserAddressModel,
	msg *mq.TransactionConfirmMessage,
	chainCode string,
	blockTime *time.Time,
	assetCode string,
	amount string,
	direction string,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if p == nil || p.svcCtx == nil || p.svcCtx.Web3TransactionRepository == nil {
		return fmt.Errorf("web3 transaction repository not configured")
	}
	if addr == nil || msg == nil {
		return nil
	}

	txHash := strings.TrimSpace(msg.TxHash)
	if txHash == "" {
		return nil
	}
	userAddress := strings.TrimSpace(addr.Address)
	if userAddress == "" {
		return nil
	}

	// tx_type: keep consistent with web3_balance_changes (respect broadcast placeholder when exists).
	fallbackTxType := p.determineTransactionType(msg, direction)
	txType := p.resolveWeb3TxType(ctx, txHash, userAddress, fallbackTxType)

	now := time.Now()
	raw := map[string]interface{}{
		"chain":                  msg.Chain,
		"transaction_type":       msg.TransactionType,
		"monitored_address":      msg.MonitoredAddress,
		"required_confirmations": msg.RequiredConfirmations,
		"log_index":              msg.LogIndex,
		"token_address":          msg.TokenAddress,
		"token_name":             msg.TokenName,
		"token_symbol":           msg.TokenSymbol,
		"token_decimals":         msg.TokenDecimals,
		"token_amount":           msg.TokenAmount,
		"value":                  msg.Value,
	}
	rawBytes, _ := json.Marshal(raw)

	// 计算 USD 价值
	amountUSD := ""
	if p.svcCtx != nil && p.svcCtx.RedisClient != nil {
		if price, ok := logic.GetAssetPrice(ctx, p.svcCtx.RedisClient, assetCode); ok && price.IsPositive() {
			// 将 amount 转换为 decimal 并计算 USD 价值
			if amountDecimal, err := decimal.NewFromString(strings.TrimSpace(amount)); err == nil && amountDecimal.IsPositive() {
				usdValue := amountDecimal.Mul(price)
				// 保留 2 位小数
				amountUSD = usdValue.Round(2).String()
				logx.Infof("✅ Calculated USD value for transaction: tx_hash=%s asset=%s amount=%s price=%s usd=%s",
					txHash, assetCode, amount, price.String(), amountUSD)
			}
		} else {
			logx.Infof("⚠️  Price not found in Redis for asset: %s, USD value will be empty", assetCode)
		}
	}

	tx := &model.Web3TransactionModel{
		Web3UserID:    addr.Web3UserID,
		UserAddress:   userAddress,
		Network:       strings.ToUpper(strings.TrimSpace(chainCode)),
		ChainID:       addr.ChainID,
		TxHash:        txHash,
		BlockNumber:   int64(msg.BlockNumber),
		BlockTime:     blockTime,
		TxType:        txType,
		Direction:     strings.TrimSpace(direction),
		AssetCode:     strings.ToUpper(strings.TrimSpace(assetCode)),
		Amount:        strings.TrimSpace(amount),
		AmountUSD:     amountUSD,
		FromAddress:   strings.TrimSpace(msg.FromAddress),
		ToAddress:     strings.TrimSpace(msg.ToAddress),
		Status:        model.Web3TxStatusConfirmed,
		Confirmations: msg.Confirmations,
		Fee: func() string {
			f, _ := commonutils.FormatWeb3Fee(chainCode, msg.GasFee, msg.GasUsed, msg.GasPrice)
			if f == nil {
				return ""
			}
			return strings.TrimSpace(*f)
		}(),
		FeeAsset: func() string {
			_, a := commonutils.FormatWeb3Fee(chainCode, msg.GasFee, msg.GasUsed, msg.GasPrice)
			if a == nil {
				return strings.TrimSpace(p.getNativeTokenSymbol(msg.Chain))
			}
			return strings.TrimSpace(*a)
		}(),
		RawData:   rawBytes,
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	return p.svcCtx.Web3TransactionRepository.Upsert(ctx, tx)
}

// findWeb2UserAddress 根据地址+链查找 Web2 用户充值地址
// 重要：同一个 EVM 地址可能同时存在于 ETH/BSC（尤其是复用同一私钥的地址），
// 若仅按 address 查询会随机命中错误链，导致入金被误判为 chain mismatch 并跳过。
func (p *TransactionConfirmProcessor) findWeb2UserAddress(ctx context.Context, address string, chain string) (*model.WalletDepositAddressModel, error) {
	if p.svcCtx == nil || p.svcCtx.WalletDepositAddressRepository == nil {
		return nil, fmt.Errorf("wallet deposit address repository not configured")
	}

	chainCode := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(chain)))
	if chainCode == "" {
		return nil, fmt.Errorf("invalid chain: %s", strings.TrimSpace(chain))
	}

	addr, err := p.svcCtx.WalletDepositAddressRepository.FindActiveByAddressAndChain(ctx, address, chainCode)
	if err != nil {
		return nil, err
	}
	return addr, nil
}

// updateExistingWeb3Transaction 更新现有 Web3 交易
func (p *TransactionConfirmProcessor) updateExistingWeb3Transaction(ctx context.Context, tx *model.Web3TransactionModel, msg *mq.TransactionConfirmMessage) error {
	// 注意：当前 TransactionConfirmMessage 不包含交易执行状态（success/revert）
	// 假设所有到达这里的交易都已在链上确认且执行成功
	// TODO: 如果 chainsync 服务未来添加 receipt.status 字段，需要在这里判断交易是否 revert
	// 示例代码：
	// txStatus := model.Web3TxStatusConfirmed
	// if msg.ReceiptStatus != nil && *msg.ReceiptStatus == 0 {
	//     txStatus = model.Web3TxStatusFailed
	// }

	// 优化：仅在状态变化或关键字段缺失时更新，避免频繁无意义的 UPDATE
	needsUpdate := false
	updates := map[string]interface{}{}

	// 检查状态是否需要更新
	if tx.Status != model.Web3TxStatusConfirmed {
		updates["status"] = model.Web3TxStatusConfirmed
		needsUpdate = true
	}

	// 检查确认数是否显著变化（避免每次 +1 都更新）
	// 策略：仅在达到关键阈值时更新（1, 6, 12, 每 10 个）
	confirmationThresholds := []int32{1, 6, 12, 20, 30, 40, 50}
	shouldUpdateConfirmations := false
	for _, threshold := range confirmationThresholds {
		if msg.Confirmations == threshold || (msg.Confirmations > 50 && msg.Confirmations%10 == 0) {
			shouldUpdateConfirmations = true
			break
		}
	}
	if shouldUpdateConfirmations && tx.Confirmations != msg.Confirmations {
		updates["confirmations"] = msg.Confirmations
		needsUpdate = true
	}

	// 更新区块信息（如果之前没有）
	if tx.BlockNumber == 0 && msg.BlockNumber > 0 {
		updates["block_number"] = msg.BlockNumber
		needsUpdate = true
	}

	// 更新区块时间：如果还没有设置区块号（说明是初始占位符时间），或者已有但消息中有更准确的时间戳
	if msg.BlockTimestamp > 0 && (tx.BlockTime == nil || tx.BlockNumber == 0) {
		blockTime := p.convertBlockTime(msg.BlockTimestamp)
		updates["block_time"] = blockTime
		needsUpdate = true
	}

	// 如果没有需要更新的字段，直接返回
	if !needsUpdate {
		logx.Debugf("Transaction already up-to-date, skipping update: tx_hash=%s confirmations=%d", msg.TxHash, msg.Confirmations)
		return nil
	}

	// 添加更新时间
	updates["updated_at"] = time.Now()

	err := p.svcCtx.Web3TransactionRepository.GetDB().WithContext(ctx).
		Model(tx).
		Where("id = ?", tx.ID).
		Updates(updates).Error

	if err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	logx.Infof("Updated transaction: tx_hash=%s, confirmations=%d, fields=%v", msg.TxHash, msg.Confirmations, updates)
	return nil
}

// syncWeb3AddressBalanceFromChain 从链上同步 Web3 地址余额到 web3_user_address_balances 表
// 同时支持主币和代币（根据 TransactionType 判断）
// 注意：余额记录是地址级别的，不绑定到特定用户/设备
func (p *TransactionConfirmProcessor) syncWeb3AddressBalanceFromChain(ctx context.Context, addr *model.Web3UserAddressModel, msg *mq.TransactionConfirmMessage) error {
	if ctx == nil || addr == nil || msg == nil {
		return nil
	}
	if p.svcCtx == nil || p.svcCtx.ChainRpc == nil || p.svcCtx.Web3UserAddressBalanceRepository == nil {
		return nil
	}

	chainType := p.chainStringToChainRpcType(msg.Chain)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return nil
	}

	address := strings.TrimSpace(addr.Address)
	if address == "" {
		return nil
	}

	chainCode := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(msg.Chain)))
	if chainCode == "" {
		return nil
	}

	var (
		assetCode     string
		balanceStr    string
		usdCents      int64
		tokenAddress  *string
		tokenDecimals int
		tokenType     string
		balanceUSD    string
		balanceUSDRaw string
		balanceRawInt int64
	)

	// Token 交易优先使用 GetTokenBalance，否则使用 GetBalance 查询主币
	if msg.IsTokenTransaction() && strings.TrimSpace(msg.TokenAddress) != "" {
		// 代币余额
		tokenAddr := strings.TrimSpace(msg.TokenAddress)
		resp, err := p.svcCtx.ChainRpc.GetTokenBalance(ctx, &pb.GetTokenBalanceReq{
			Chain:         chainType,
			Address:       address,
			TokenContract: tokenAddr,
		})
		if err != nil {
			logx.Errorf("GetTokenBalance RPC error: address=%s token=%s chain=%s error=%v", address, tokenAddr, msg.Chain, err)
			// Fallback: some TRON nodes don't support constant contract calls (TriggerConstantContract).
			// In that case, we still want balances to move forward based on confirmed transfer deltas.
			if fbErr := p.applyTokenBalanceDeltaFallback(ctx, addr, msg, chainCode, tokenAddr); fbErr != nil {
				return fmt.Errorf("get token balance failed: %w", err)
			}
			return nil
		}
		if resp == nil {
			logx.Errorf("GetTokenBalance returned nil response: address=%s token=%s chain=%s", address, tokenAddr, msg.Chain)
			if fbErr := p.applyTokenBalanceDeltaFallback(ctx, addr, msg, chainCode, tokenAddr); fbErr != nil {
				return fmt.Errorf("get token balance returned nil response")
			}
			return nil
		}
		if !resp.Success {
			logx.Errorf("GetTokenBalance failed: address=%s token=%s chain=%s message=%s", address, tokenAddr, msg.Chain, resp.Message)
			if fbErr := p.applyTokenBalanceDeltaFallback(ctx, addr, msg, chainCode, tokenAddr); fbErr != nil {
				return fmt.Errorf("get token balance failed: %s", resp.Message)
			}
			return nil
		}

		balanceStr = strings.TrimSpace(resp.Balance)
		if balanceStr == "" {
			balanceStr = "0"
		}
		if resp.BalanceUsd > 0 {
			usdCents = int64(math.Round(resp.BalanceUsd * 100))
		}
		if usdCents < 0 {
			usdCents = 0
		}
		assetCode = strings.ToUpper(strings.TrimSpace(resp.TokenSymbol))
		if assetCode == "" || assetCode == "UNKNOWN" {
			assetCode = p.determineAssetCode(ctx, msg)
			if assetCode == "" {
				logx.Errorf("Cannot determine asset code for token balance sync: address=%s chain=%s contract=%s",
					address, msg.Chain, msg.TokenAddress)
				return fmt.Errorf("cannot determine asset code for token")
			}
		}
		// Balance returned by ChainRPC is on-chain raw smallest unit. Use on-chain decimals to interpret it.
		tokenDecimals = int(resp.TokenDecimals)
		if tokenDecimals <= 0 {
			// Fallback to the event message (chainsync already queried decimals for token transfers).
			tokenDecimals = int(msg.TokenDecimals)
		}
		if tokenDecimals <= 0 && p.svcCtx != nil {
			// Last resort fallback; should rarely happen.
			if item := logic.GetAssetInfoWithCache(ctx, p.svcCtx, assetCode); item != nil && item.Precision > 0 {
				tokenDecimals = int(item.Precision)
			}
		}
		if tokenDecimals <= 0 {
			tokenDecimals = 18
		}
		tokenType = "token"
		tokenAddress = &tokenAddr
	} else {
		// 主币余额
		resp, err := p.svcCtx.ChainRpc.GetBalance(ctx, &pb.GetBalanceReq{
			Chain:   chainType,
			Address: address,
		})
		if err != nil {
			logx.Errorf("GetBalance RPC error: address=%s chain=%s error=%v", address, msg.Chain, err)
			return fmt.Errorf("get balance failed: %w", err)
		}
		if resp == nil {
			logx.Errorf("GetBalance returned nil response: address=%s chain=%s", address, msg.Chain)
			return fmt.Errorf("get balance returned nil response")
		}
		if !resp.Success {
			logx.Errorf("GetBalance failed: address=%s chain=%s message=%s", address, msg.Chain, resp.Message)
			return fmt.Errorf("get balance failed: %s", resp.Message)
		}
		balanceStr = strings.TrimSpace(resp.Balance)
		if balanceStr == "" {
			balanceStr = "0"
		}
		if resp.BalanceUsd > 0 {
			usdCents = int64(math.Round(resp.BalanceUsd * 100))
		}
		if usdCents < 0 {
			usdCents = 0
		}
		assetCode = p.getNativeTokenSymbol(msg.Chain)
		if assetCode == "" || assetCode == "UNKNOWN" {
			assetCode = p.determineAssetCode(ctx, msg)
		}
		// 主币精度：优先使用后台配置的精度（从 asset 表获取）
		tokenDecimals = 0
		if p.svcCtx != nil {
			if item := logic.GetAssetInfoWithCache(ctx, p.svcCtx, assetCode); item != nil && item.Precision > 0 {
				tokenDecimals = int(item.Precision)
			}
		}
		// 如果后台配置的精度无效，使用默认精度
		if tokenDecimals == 0 {
			switch strings.ToUpper(chainCode) {
			case "ETH":
				tokenDecimals = 18
			case "BSC":
				tokenDecimals = 18
			case "TRON":
				tokenDecimals = 6
			default:
				tokenDecimals = 18
			}
		}
		tokenType = "native"
		tokenAddress = nil
	}

	// 如果 ChainRpc 没有返回 USD 估值，则从 Redis 获取价格自行计算
	if usdCents <= 0 {
		if p.svcCtx.RedisClient == nil {
			logx.Infof("⚠️  USD value is 0 and Redis not available: asset=%s address=%s", assetCode, address)
		} else {
			// 从 Redis 获取当前价格（market 服务写入）
			if price, ok := logic.GetAssetPrice(ctx, p.svcCtx.RedisClient, assetCode); ok && price.IsPositive() {
				// 将原始余额转换为实际余额（使用精度）
				balanceDecimal, err := decimal.NewFromString(balanceStr)
				if err == nil && balanceDecimal.IsPositive() {
					decimalsDivisor := decimal.NewFromInt(1).Shift(int32(tokenDecimals))
					actualBalance := balanceDecimal.Div(decimalsDivisor)
					// 计算 USD 价值：actualBalance * price_usd
					if actualBalance.IsPositive() {
						usdValue := actualBalance.Mul(price)
						// 转换为分（cents）
						usdCents = usdValue.Mul(decimal.NewFromInt(100)).IntPart()
						logx.Debugf("Calculated USD value from Redis price: asset=%s balance=%s price=%s usd_cents=%d",
							assetCode, actualBalance.String(), price.String(), usdCents)
					}
				}
			} else {
				logx.Infof("⚠️  Price not found in Redis for asset: %s, USD value will be 0", assetCode)
			}
		}
	}

	// 解析原始余额为 int64（仅用于聚合排序）
	// 注意：对于超大余额（如某些代币的总供应量），int64 会溢出
	// 这里使用安全转换，超出范围时使用 MaxInt64
	if bi, ok := new(big.Int).SetString(balanceStr, 10); ok {
		if bi.IsInt64() {
			balanceRawInt = bi.Int64()
		} else {
			// 超出 int64 范围，使用最大值（保持排序一致性）
			balanceRawInt = 9223372036854775807 // math.MaxInt64
			logx.Infof("Balance exceeds int64 range, using MaxInt64 for sorting: balance=%s asset=%s", balanceStr, assetCode)
		}
	}

	// 构建 USD 字段
	if usdCents < 0 {
		usdCents = 0
	}
	balanceUSDRaw = strconv.FormatInt(usdCents, 10)
	balanceUSD = formatUSDFromCents(usdCents)

	db := p.svcCtx.Web3UserAddressBalanceRepository.GetDB()
	if db == nil {
		return nil
	}

	now := time.Now()
	var existing model.Web3UserAddressBalanceModel
	err := db.WithContext(ctx).
		Where("wallet_address = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL",
			address, assetCode, chainCode).
		First(&existing).Error

	switch err {
	case nil:
		// 更新现有记录
		if err := p.svcCtx.Web3UserAddressBalanceRepository.UpdateBalance(
			ctx,
			existing.ID,
			balanceStr,
			balanceRawInt,
			balanceUSD,
			usdCents,
		); err != nil {
			return err
		}
		// Ensure token_decimals stays consistent with raw units.
		if msg.IsTokenTransaction() && existing.TokenDecimals != tokenDecimals {
			_ = db.WithContext(ctx).
				Model(&model.Web3UserAddressBalanceModel{}).
				Where("id = ?", existing.ID).
				Update("token_decimals", tokenDecimals).Error
		}
		return nil
	case gorm.ErrRecordNotFound:
		// 创建新记录（不绑定到特定用户/设备）
		newBalance := &model.Web3UserAddressBalanceModel{
			AssetCode:        assetCode,
			ChainCode:        chainCode,
			WalletAddress:    address,
			Network:          chainCode, // 使用 chainCode（ETH/BSC/TRON）而不是 addr.Network，确保与前端查询一致
			ChainID:          addr.ChainID,
			Balance:          balanceStr,
			BalanceRaw:       balanceStr,
			BalanceUSD:       balanceUSD,
			BalanceUSDRaw:    balanceUSDRaw,
			TokenAddress:     tokenAddress,
			TokenDecimals:    tokenDecimals,
			TokenType:        tokenType,
			IsVerified:       0,
			IsSpam:           0,
			BalanceUpdatedAt: &now,
			LastSyncedAt:     &now,
			SyncStatus:       "synced",
			SyncError:        nil,
		}
		return p.svcCtx.Web3UserAddressBalanceRepository.Create(ctx, newBalance)
	default:
		return err
	}
}

// applyTokenBalanceDeltaFallback updates token balances using confirmed transfer deltas (token_amount),
// without calling ChainRPC.GetTokenBalance. This is used when token balance RPC is unavailable (e.g. TRON node
// doesn't support constant contract calls).
//
// Assumptions:
// - msg.TokenAmount is on-chain raw smallest unit (integer string)
// - msg.Direction is relative to msg.MonitoredAddress (in/out)
func (p *TransactionConfirmProcessor) applyTokenBalanceDeltaFallback(
	ctx context.Context,
	addr *model.Web3UserAddressModel,
	msg *mq.TransactionConfirmMessage,
	chainCode string,
	tokenContract string,
) error {
	if ctx == nil || p == nil || p.svcCtx == nil || p.svcCtx.Web3UserAddressBalanceRepository == nil {
		return fmt.Errorf("balance repository not configured")
	}
	if addr == nil || msg == nil {
		return fmt.Errorf("nil addr/msg")
	}
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return fmt.Errorf("chain_code required")
	}
	address := strings.TrimSpace(addr.Address)
	if address == "" {
		return fmt.Errorf("address required")
	}

	// Determine asset code & decimals from the event itself (chainsync already parsed token symbol/decimals).
	assetCode := strings.ToUpper(strings.TrimSpace(msg.TokenSymbol))
	if assetCode == "" || assetCode == "UNKNOWN" {
		assetCode = p.determineAssetCode(ctx, msg)
	}
	if assetCode == "" {
		return fmt.Errorf("cannot determine asset code for token delta fallback")
	}
	decimals := int(msg.TokenDecimals)
	if decimals <= 0 {
		decimals = 18
	}

	rawDeltaStr := strings.TrimSpace(msg.TokenAmount)
	if rawDeltaStr == "" {
		// As a last resort, fall back to msg.Value (already decimal) * 10^decimals.
		rawDeltaStr = ""
	}

	deltaBI, ok := new(big.Int).SetString(rawDeltaStr, 10)
	if !ok || deltaBI.Sign() < 0 {
		return fmt.Errorf("invalid token_amount for delta fallback: %q", msg.TokenAmount)
	}

	// Direction: prefer msg.Direction; fallback to address match.
	dir := normalizeTxDirection(msg.Direction)
	if dir != "in" && dir != "out" {
		if strings.TrimSpace(msg.ToAddress) == address {
			dir = "in"
		} else if strings.TrimSpace(msg.FromAddress) == address {
			dir = "out"
		}
	}
	if dir != "in" && dir != "out" {
		return fmt.Errorf("cannot determine direction for delta fallback: %q", msg.Direction)
	}

	db := p.svcCtx.Web3UserAddressBalanceRepository.GetDB()
	if db == nil {
		return fmt.Errorf("db is nil")
	}

	var existing model.Web3UserAddressBalanceModel
	findErr := db.WithContext(ctx).
		Where("wallet_address = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL",
			address, assetCode, chainCode).
		First(&existing).Error

	now := time.Now()

	applyDelta := func(oldRaw string) (string, error) {
		oldBI := big.NewInt(0)
		if strings.TrimSpace(oldRaw) != "" {
			if bi, ok := new(big.Int).SetString(strings.TrimSpace(oldRaw), 10); ok && bi.Sign() >= 0 {
				oldBI = bi
			}
		}
		newBI := new(big.Int).Set(oldBI)
		if dir == "in" {
			newBI.Add(newBI, deltaBI)
		} else {
			newBI.Sub(newBI, deltaBI)
			if newBI.Sign() < 0 {
				newBI.SetInt64(0)
			}
		}
		return newBI.String(), nil
	}

	switch findErr {
	case nil:
		oldRaw := strings.TrimSpace(existing.BalanceRaw)
		if oldRaw == "" {
			oldRaw = strings.TrimSpace(existing.Balance)
		}
		newRaw, err := applyDelta(oldRaw)
		if err != nil {
			return err
		}
		contract := strings.TrimSpace(tokenContract)
		var contractPtr *string
		if contract != "" {
			contractPtr = &contract
		}
		return db.WithContext(ctx).
			Model(&model.Web3UserAddressBalanceModel{}).
			Where("id = ?", existing.ID).
			Updates(map[string]interface{}{
				"balance":            newRaw,
				"balance_raw":        newRaw,
				"token_address":      contractPtr,
				"token_decimals":     decimals,
				"token_type":         "token",
				"balance_updated_at": &now,
				"last_synced_at":     &now,
				"sync_status":        "synced",
				"sync_error":         nil,
			}).Error
	case gorm.ErrRecordNotFound:
		newRaw, err := applyDelta("0")
		if err != nil {
			return err
		}
		contract := strings.TrimSpace(tokenContract)
		var contractPtr *string
		if contract != "" {
			contractPtr = &contract
		}
		m := &model.Web3UserAddressBalanceModel{
			AssetCode:        assetCode,
			ChainCode:        chainCode,
			WalletAddress:    address,
			Network:          chainCode,
			ChainID:          addr.ChainID,
			Balance:          newRaw,
			BalanceRaw:       newRaw,
			BalanceUSD:       "0",
			BalanceUSDRaw:    "0",
			TokenAddress:     contractPtr,
			TokenDecimals:    decimals,
			TokenType:        "token",
			IsVerified:       0,
			IsSpam:           0,
			BalanceUpdatedAt: &now,
			LastSyncedAt:     &now,
			SyncStatus:       "synced",
			SyncError:        nil,
		}
		return p.svcCtx.Web3UserAddressBalanceRepository.Create(ctx, m)
	default:
		return findErr
	}
}

// createWeb3Transaction 创建新 Web3 交易记录
func (p *TransactionConfirmProcessor) createWeb3Transaction(
	ctx context.Context,
	addr *model.Web3UserAddressModel,
	msg *mq.TransactionConfirmMessage,
	direction, txType, assetCode, amount string,
	blockTime *time.Time,
) error {
	now := time.Now()
	// 获取标准化的 chainCode（ETH/BSC/TRON）确保与前端查询一致
	networkCode := p.chainStringToChainCode(msg.Chain)
	tx := &model.Web3TransactionModel{
		Web3UserID:    addr.Web3UserID,
		UserAddress:   addr.Address,
		Network:       networkCode, // 使用标准化的 chainCode（ETH/BSC/TRON）而不是 addr.Network
		ChainID:       addr.ChainID,
		TxHash:        msg.TxHash,
		BlockNumber:   int64(msg.BlockNumber),
		BlockTime:     blockTime,
		TxType:        txType,
		Direction:     direction,
		AssetCode:     assetCode,
		Amount:        amount,
		FromAddress:   msg.FromAddress,
		ToAddress:     msg.ToAddress,
		Status:        model.Web3TxStatusConfirmed,
		Confirmations: msg.Confirmations,
		Fee:           msg.GasFee,
		FeeAsset:      p.getNativeTokenSymbol(msg.Chain),
		CreatedAt:     &now,
		UpdatedAt:     &now,
	}

	// 如果有代币信息，可以存储到 raw_data
	if msg.IsTokenTransaction() {
		rawData := map[string]interface{}{
			"token_address":  msg.TokenAddress,
			"token_name":     msg.TokenName,
			"token_symbol":   msg.TokenSymbol,
			"token_decimals": msg.TokenDecimals,
			"log_index":      msg.LogIndex,
		}
		// TODO: 将 rawData 序列化为 JSON 存储到 raw_data 字段
		_ = rawData
	}

	if err := p.svcCtx.Web3TransactionRepository.Create(ctx, tx); err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	logx.Infof("Created Web3 transaction: tx_hash=%s, user_id=%d, direction=%s, amount=%s %s",
		msg.TxHash, addr.Web3UserID, direction, amount, assetCode)
	return nil
}

// createWeb2Transaction 创建新 Web2 交易记录
func (p *TransactionConfirmProcessor) resolveDepositAssetCode(ctx context.Context, chainCode string, msg *mq.TransactionConfirmMessage) (string, error) {
	if msg == nil {
		return "", fmt.Errorf("nil msg")
	}
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	if chainCode == "" {
		return "", fmt.Errorf("chain_code required")
	}

	// Token deposit: map contract_address -> asset_code via currency_chain_settings (authoritative in admin).
	if msg.IsTokenTransaction() {
		contract := strings.TrimSpace(msg.TokenAddress)
		if contract != "" && p.svcCtx != nil && p.svcCtx.CurrencyChainSettingsRepository != nil {
			var mapping model.CurrencyChainSettingsModel
			q := p.svcCtx.CurrencyChainSettingsRepository.GetDB().WithContext(ctx).
				Select("asset_code").
				Model(&model.CurrencyChainSettingsModel{}).
				Where("chain_code = ? AND status = 1 AND deleted_at IS NULL AND deposit_enabled = 1", chainCode)

			// EVM contract addresses are case-insensitive; normalize for lookup.
			if strings.HasPrefix(strings.ToLower(contract), "0x") {
				q = q.Where("LOWER(contract_address) = ?", strings.ToLower(contract))
			} else {
				q = q.Where("contract_address = ?", contract)
			}

			if err := q.First(&mapping).Error; err != nil {
				if err != gorm.ErrRecordNotFound {
					return "", fmt.Errorf("lookup token mapping failed: %w", err)
				}
			} else {
				code := strings.ToUpper(strings.TrimSpace(mapping.AssetCode))
				if code != "" {
					return code, nil
				}
			}
		}

		// Fallback: token_symbol (best-effort only).
		if s := strings.TrimSpace(msg.TokenSymbol); s != "" {
			return strings.ToUpper(s), nil
		}
		return "", nil
	}

	// Native deposit.
	native := strings.ToUpper(strings.TrimSpace(p.getNativeTokenSymbol(chainCode)))
	if native == "" || native == "UNKNOWN" {
		return "", nil
	}
	return native, nil
}

func normalizeDepositAmountDecimal(msg *mq.TransactionConfirmMessage, assetPrecision int32) (string, error) {
	if msg == nil {
		return "", fmt.Errorf("nil msg")
	}
	if assetPrecision < 0 || assetPrecision > 30 {
		return "", fmt.Errorf("invalid asset precision: %d", assetPrecision)
	}

	normalizeFromDecimalUnits := func(s string) (string, error) {
		raw, err := accounting.ParseDecimalToRawExact(s, assetPrecision, false)
		if err != nil {
			return "", err
		}
		return accounting.RawToDecimalString(raw, assetPrecision)
	}

	// Token: upstream must pass TokenDecimals; otherwise we cannot safely interpret TokenAmount (chain raw vs asset raw).
	if msg.IsTokenTransaction() {
		if msg.TokenDecimals == 0 {
			return "", fmt.Errorf("token_decimals required for token transaction (tx_hash=%s chain=%s token_amount=%s)",
				strings.TrimSpace(msg.TxHash), strings.TrimSpace(msg.Chain), strings.TrimSpace(msg.TokenAmount))
		}
		if isFormattedValue(msg.Value) {
			if s := extractNumericAmount(msg.Value); s != "" {
				return normalizeFromDecimalUnits(s)
			}
		}
		if s := strings.TrimSpace(msg.TokenAmount); s != "" {
			// If TokenAmount already looks like a decimal in asset units, treat it as such.
			if strings.Contains(s, ".") {
				return normalizeFromDecimalUnits(s)
			}
			// Otherwise TokenAmount is raw smallest-unit integer (from on-chain logs); TokenDecimals is required and already checked above.
			rawTokenInt, err := parseRawIntDecimal65(s)
			if err != nil {
				return "", err
			}
			td := int32(msg.TokenDecimals)
			var rawAssetInt decimal.Decimal
			switch {
			case td == assetPrecision:
				rawAssetInt = rawTokenInt
			case td > assetPrecision:
				divisor := decimal.New(1, td-assetPrecision)
				rawAssetInt = rawTokenInt.Div(divisor).Truncate(0)
			default:
				multiplier := decimal.New(1, assetPrecision-td)
				rawAssetInt = rawTokenInt.Mul(multiplier)
			}
			return accounting.RawToDecimalString(rawAssetInt, assetPrecision)
		}
		return "", fmt.Errorf("amount required")
	}

	// Native: ChainSync's tx.Value is ALWAYS formatted by units.FormatTokenAmount (already in asset units).
	// Example: 1 TRX (1,000,000 sun) -> "1" or "1.5" depending on whether remainder exists.
	// We should NOT treat it as raw smallest-unit integer again.
	v := strings.TrimSpace(msg.Value)
	if v == "" {
		return "", fmt.Errorf("amount required")
	}
	if isFormattedValue(v) {
		if s := extractNumericAmount(v); s != "" {
			return normalizeFromDecimalUnits(s)
		}
	}
	// ChainSync always formats native coin amounts via FormatTokenAmount, so treat all values as decimal units.
	// This includes both "1.5" (with decimal) and "1" (integer, no decimal point).
	return normalizeFromDecimalUnits(v)
}

func parseRawIntDecimal65(s string) (decimal.Decimal, error) {
	// NOTE: keep this wrapper to centralize integer/size validation.
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, fmt.Errorf("raw required")
	}
	if strings.ContainsAny(s, ".eE") {
		return decimal.Zero, fmt.Errorf("raw must be integer")
	}

	raw, err := accounting.ParseDecimalToRawExact(s, 0, true)
	if err != nil {
		return decimal.Zero, err
	}
	return raw, nil
}

func extractNumericAmount(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func isFormattedValue(s string) bool {
	parts := strings.Fields(strings.TrimSpace(s))
	return len(parts) >= 2
}

func sameChainAddress(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(a), "0x") || strings.HasPrefix(strings.ToLower(b), "0x") {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func depositConfirmIdemKeyAndBizRef(txHash string) (string, string) {
	txHash = strings.TrimSpace(txHash)
	sum := sha256.Sum256([]byte(txHash))
	hashHex := hex.EncodeToString(sum[:])
	return "deposit:confirm:" + hashHex, "deposit_tx:" + hashHex
}

func (p *TransactionConfirmProcessor) upsertWalletDepositRecord(
	ctx context.Context,
	addr *model.WalletDepositAddressModel,
	msg *mq.TransactionConfirmMessage,
	chainCode string,
	assetCode string,
	amountDecimal string,
) error {
	if p.svcCtx == nil || p.svcCtx.DB == nil {
		return nil
	}
	txHash := strings.TrimSpace(msg.TxHash)
	if txHash == "" {
		return fmt.Errorf("tx_hash required")
	}

	var existing struct {
		ID       int64
		UserID   int64
		Asset    string
		Chain    string
		Status   string
		Deleted  *time.Time
		Amount   string
		Address  string
		TxHash   string
		BlockNum *int64
	}
	err := p.svcCtx.DB.WithContext(ctx).
		Table("wallet_deposits").
		Select("id,user_id,asset_code,chain_code,status").
		Where("transaction_hash = ? AND deleted_at IS NULL", txHash).
		Take(&existing).Error

	blockNumber := any(nil)
	if msg.BlockNumber > 0 {
		v := int64(msg.BlockNumber)
		blockNumber = &v
	}

	memo := any(nil)
	if addr.Memo != nil && strings.TrimSpace(*addr.Memo) != "" {
		m := strings.TrimSpace(*addr.Memo)
		memo = &m
	}

	fields := map[string]interface{}{
		"user_id":          addr.UserID,
		"asset_code":       strings.ToUpper(strings.TrimSpace(assetCode)),
		"chain_code":       strings.ToUpper(strings.TrimSpace(chainCode)),
		"deposit_address":  strings.TrimSpace(addr.Address),
		"amount":           strings.TrimSpace(amountDecimal),
		"transaction_hash": txHash,
		"block_number":     blockNumber,
		"confirmations":    msg.Confirmations,
		"status":           "completed",
		"memo":             memo,
	}

	updateLastUsedAt := func() {
		if addr == nil || addr.ID <= 0 {
			return
		}
		now := time.Now()
		_ = p.svcCtx.DB.WithContext(ctx).
			Table("wallet_user_chain_addresses").
			Where("id = ? AND deleted_at IS NULL", addr.ID).
			Updates(map[string]interface{}{
				"last_used_at": &now,
				"updated_at":   &now,
			}).Error
	}

	switch err {
	case nil:
		err := p.svcCtx.DB.WithContext(ctx).
			Table("wallet_deposits").
			Where("id = ? AND deleted_at IS NULL", existing.ID).
			Updates(fields).Error
		if err == nil {
			updateLastUsedAt()
		}
		return err
	case gorm.ErrRecordNotFound:
		err := p.svcCtx.DB.WithContext(ctx).Table("wallet_deposits").Create(fields).Error
		if err == nil {
			updateLastUsedAt()
		}
		return err
	default:
		return fmt.Errorf("query wallet_deposits failed: %w", err)
	}
}

// chainStringToChainCode 将 chainsync 的 chain 字符串转换为 Web2 的 chain_code
func (p *TransactionConfirmProcessor) chainStringToChainCode(chain string) string {
	switch strings.ToUpper(chain) {
	case "CHAIN_TYPE_ETHEREUM", "ETHEREUM", "ETH":
		return "ETH"
	case "CHAIN_TYPE_BSC", "BSC":
		return "BSC"
	case "CHAIN_TYPE_TRON", "TRON", "TRX":
		return "TRON"
	default:
		return strings.ToUpper(chain)
	}
}

// chainStringToChainID 将链字符串转换为 chainId
// TODO: 应从配置文件读取，区分主网/测试网环境
func (p *TransactionConfirmProcessor) chainStringToChainID(chain string) int64 {
	// 优先从环境变量读取（格式：CHAIN_ID_ETH=1, CHAIN_ID_BSC=56, CHAIN_ID_TRON=728126428）
	if envChainID := os.Getenv("CHAIN_ID_" + strings.ToUpper(p.chainStringToChainCode(chain))); envChainID != "" {
		if id, err := strconv.ParseInt(envChainID, 10, 64); err == nil && id > 0 {
			return id
		}
	}

	// 默认使用主网 chainId
	switch strings.ToUpper(chain) {
	case "CHAIN_TYPE_ETHEREUM", "ETHEREUM", "ETH":
		return 1 // Ethereum 主网（测试网 Sepolia: 11155111）
	case "CHAIN_TYPE_BSC", "BSC":
		return 56 // BSC 主网（测试网: 97）
	case "CHAIN_TYPE_TRON", "TRON", "TRX":
		// TRON 主网没有标准 EIP-155 chainId，使用自定义标识
		return 728126428
	default:
		logx.Infof("⚠️  Unknown chain type for chainId mapping: %s", chain)
		return 0
	}
}

// getNativeTokenSymbol 获取主币符号
func (p *TransactionConfirmProcessor) getNativeTokenSymbol(chain string) string {
	switch strings.ToUpper(chain) {
	case "CHAIN_TYPE_ETHEREUM", "ETHEREUM", "ETH":
		return "ETH"
	case "CHAIN_TYPE_BSC", "BSC":
		return "BNB"
	case "CHAIN_TYPE_TRON", "TRON", "TRX":
		return "TRX"
	default:
		return "UNKNOWN"
	}
}

// inferTokenSymbol 根据链和合约地址推断代币符号
// 从 currency_chain_settings 表查询代币信息
func (p *TransactionConfirmProcessor) inferTokenSymbol(ctx context.Context, chain, tokenAddress string) string {
	if p.svcCtx == nil || p.svcCtx.CurrencyChainSettingsRepository == nil {
		logx.Infof("⚠️  Cannot infer token symbol (repository not configured): chain=%s contract=%s", chain, tokenAddress)
		return ""
	}

	if ctx == nil {
		ctx = context.Background()
	}

	chainCode := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(chain)))
	tokenAddress = strings.TrimSpace(tokenAddress)

	if chainCode == "" || tokenAddress == "" {
		return ""
	}

	// 查询 currency_chain_settings 表
	db := p.svcCtx.CurrencyChainSettingsRepository.GetDB()
	if db == nil {
		return ""
	}

	var setting model.CurrencyChainSettingsModel
	query := db.WithContext(ctx).
		Select("asset_code").
		Model(&model.CurrencyChainSettingsModel{}).
		Where("chain_code = ? AND status = 1 AND deleted_at IS NULL", chainCode)

	// EVM 合约地址不区分大小写
	if strings.HasPrefix(strings.ToLower(tokenAddress), "0x") {
		query = query.Where("LOWER(contract_address) = ?", strings.ToLower(tokenAddress))
	} else {
		// TRON 地址区分大小写
		query = query.Where("contract_address = ?", tokenAddress)
	}

	err := query.First(&setting).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			logx.Errorf("Failed to query token symbol from currency_chain_settings: chain=%s contract=%s error=%v",
				chainCode, tokenAddress, err)
		} else {
			logx.Infof("⚠️  Token not found in currency_chain_settings: chain=%s contract=%s",
				chainCode, tokenAddress)
		}
		return ""
	}

	assetCode := strings.ToUpper(strings.TrimSpace(setting.AssetCode))
	if assetCode == "" {
		logx.Infof("⚠️  Empty asset_code in currency_chain_settings: chain=%s contract=%s",
			chainCode, tokenAddress)
		return ""
	}

	logx.Infof("✅ Resolved token symbol from database: chain=%s contract=%s => %s",
		chainCode, tokenAddress, assetCode)
	return assetCode
}

// formatUSDFromCents 将“分”转换为 USD 字符串
func formatUSDFromCents(cents int64) string {
	if cents == 0 {
		return "0"
	}
	dollars := float64(cents) / 100.0
	s := strconv.FormatFloat(dollars, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// syncWeb3NativeBalanceFromChain 同步主币余额（用于代币交易后的 gas 消耗更新）
func (p *TransactionConfirmProcessor) syncWeb3NativeBalanceFromChain(ctx context.Context, addr *model.Web3UserAddressModel, msg *mq.TransactionConfirmMessage) error {
	if ctx == nil || addr == nil || msg == nil {
		return nil
	}
	if p.svcCtx == nil || p.svcCtx.ChainRpc == nil || p.svcCtx.Web3UserAddressBalanceRepository == nil {
		return nil
	}

	chainType := p.chainStringToChainRpcType(msg.Chain)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return nil
	}

	address := strings.TrimSpace(addr.Address)
	if address == "" {
		return nil
	}

	chainCode := strings.ToUpper(strings.TrimSpace(p.chainStringToChainCode(msg.Chain)))
	if chainCode == "" {
		return nil
	}

	// 查询主币余额
	resp, err := p.svcCtx.ChainRpc.GetBalance(ctx, &pb.GetBalanceReq{
		Chain:   chainType,
		Address: address,
	})
	if err != nil {
		logx.Errorf("GetBalance RPC error for native balance: address=%s chain=%s error=%v", address, msg.Chain, err)
		return fmt.Errorf("get native balance failed: %w", err)
	}
	if resp == nil {
		logx.Errorf("GetBalance returned nil response for native balance: address=%s chain=%s", address, msg.Chain)
		return fmt.Errorf("get native balance returned nil response")
	}
	if !resp.Success {
		logx.Errorf("GetBalance failed for native balance: address=%s chain=%s message=%s", address, msg.Chain, resp.Message)
		return fmt.Errorf("get native balance failed: %s", resp.Message)
	}

	balanceStr := strings.TrimSpace(resp.Balance)
	if balanceStr == "" {
		balanceStr = "0"
	}

	var usdCents int64
	if resp.BalanceUsd > 0 {
		usdCents = int64(math.Round(resp.BalanceUsd * 100))
	}
	if usdCents < 0 {
		usdCents = 0
	}

	assetCode := p.getNativeTokenSymbol(msg.Chain)
	if assetCode == "" || assetCode == "UNKNOWN" {
		return fmt.Errorf("cannot determine native token symbol for chain: %s", msg.Chain)
	}

	// 主币精度：优先使用后台配置的精度（从 asset 表获取）
	var tokenDecimals int
	if p.svcCtx != nil {
		if item := logic.GetAssetInfoWithCache(ctx, p.svcCtx, assetCode); item != nil && item.Precision > 0 {
			tokenDecimals = int(item.Precision)
		}
	}
	// 如果后台配置的精度无效，使用默认精度
	if tokenDecimals == 0 {
		switch strings.ToUpper(chainCode) {
		case "ETH":
			tokenDecimals = 18
		case "BSC":
			tokenDecimals = 18
		case "TRON":
			tokenDecimals = 6
		default:
			tokenDecimals = 18
		}
	}

	// 如果 ChainRpc 没有返回 USD 估值，则自己计算
	if usdCents <= 0 && p.svcCtx.RedisClient != nil {
		if price, ok := logic.GetAssetPrice(ctx, p.svcCtx.RedisClient, assetCode); ok && price.IsPositive() {
			balanceDecimal, err := decimal.NewFromString(balanceStr)
			if err == nil && balanceDecimal.IsPositive() {
				decimalsDivisor := decimal.NewFromInt(1).Shift(int32(tokenDecimals))
				actualBalance := balanceDecimal.Div(decimalsDivisor)
				if actualBalance.IsPositive() {
					usdValue := actualBalance.Mul(price)
					usdCents = usdValue.Mul(decimal.NewFromInt(100)).IntPart()
				}
			}
		}
	}

	// 解析原始余额为 int64
	var balanceRawInt int64
	if bi, ok := new(big.Int).SetString(balanceStr, 10); ok {
		if bi.IsInt64() {
			balanceRawInt = bi.Int64()
		} else {
			// 超出 int64 范围，使用最大值
			balanceRawInt = 9223372036854775807
		}
	}

	if usdCents < 0 {
		usdCents = 0
	}
	balanceUSD := formatUSDFromCents(usdCents)

	// 更新或创建余额记录
	db := p.svcCtx.Web3UserAddressBalanceRepository.GetDB()
	if db == nil {
		return nil
	}

	now := time.Now()
	var existing model.Web3UserAddressBalanceModel
	err = db.WithContext(ctx).
		Where("wallet_address = ? AND asset_code = ? AND chain_code = ? AND deleted_at IS NULL",
			address, assetCode, chainCode).
		First(&existing).Error

	switch err {
	case nil:
		// 更新现有记录
		if err := p.svcCtx.Web3UserAddressBalanceRepository.UpdateBalance(
			ctx,
			existing.ID,
			balanceStr,
			balanceRawInt,
			balanceUSD,
			usdCents,
		); err != nil {
			return err
		}
		// Ensure token_decimals stays consistent with raw units.
		if msg.IsTokenTransaction() && existing.TokenDecimals != tokenDecimals {
			_ = db.WithContext(ctx).
				Model(&model.Web3UserAddressBalanceModel{}).
				Where("id = ?", existing.ID).
				Update("token_decimals", tokenDecimals).Error
		}
		return nil
	case gorm.ErrRecordNotFound:
		// 创建新记录
		newBalance := &model.Web3UserAddressBalanceModel{
			AssetCode:        assetCode,
			ChainCode:        chainCode,
			WalletAddress:    address,
			Network:          chainCode, // 使用 chainCode（ETH/BSC/TRON）而不是 addr.Network，确保与前端查询一致
			ChainID:          addr.ChainID,
			Balance:          balanceStr,
			BalanceRaw:       balanceStr,
			BalanceUSD:       balanceUSD,
			BalanceUSDRaw:    strconv.FormatInt(usdCents, 10),
			TokenAddress:     nil,
			TokenDecimals:    tokenDecimals,
			TokenType:        "native",
			IsVerified:       0,
			IsSpam:           0,
			BalanceUpdatedAt: &now,
			LastSyncedAt:     &now,
			SyncStatus:       "synced",
			SyncError:        nil,
		}
		return p.svcCtx.Web3UserAddressBalanceRepository.Create(ctx, newBalance)
	default:
		return err
	}
}

// updateDepositAddressBalanceViaRPC 更新充值地址余额（使用本地事务）
// 注意：此方法在单个数据库事务中完成余额更新和历史记录，确保原子性
func (p *TransactionConfirmProcessor) updateDepositAddressBalanceViaRPC(
	ctx context.Context,
	addr *model.WalletDepositAddressModel,
	msg *mq.TransactionConfirmMessage,
	chainCode string,
	assetCode string,
	depositAmountDecimal string,
	assetPrecision int32,
) error {
	if p.svcCtx == nil || p.svcCtx.DB == nil {
		return fmt.Errorf("database not configured")
	}
	if p.svcCtx.WalletDepositAddressBalanceRepository == nil {
		return fmt.Errorf("balance repository not configured")
	}
	if p.svcCtx.WalletDepositAddressBalanceChangeRepository == nil {
		return fmt.Errorf("balance change repository not configured")
	}

	logx.Infof("🔄 Processing deposit address balance update: tx_hash=%s deposit_addr_id=%d user_id=%d asset=%s amount=%s",
		msg.TxHash, addr.ID, addr.UserID, assetCode, depositAmountDecimal)

	// 1. 计算充值金额的原始值（最小单位）
	depositAmountRaw, err := accounting.ParseDecimalToRawExact(depositAmountDecimal, assetPrecision, false)
	if err != nil {
		logx.Errorf("❌ Failed to parse deposit amount: amount=%s precision=%d error=%v", depositAmountDecimal, assetPrecision, err)
		return fmt.Errorf("parse deposit amount failed: %w", err)
	}
	logx.Infof("   📊 Deposit amount (raw): %s", depositAmountRaw.String())

	// 2. 幂等性检查：先检查历史记录是否已存在（在事务外检查，避免锁表）
	txHash := strings.TrimSpace(msg.TxHash)
	if existingChange, _ := p.svcCtx.WalletDepositAddressBalanceChangeRepository.FindByTxHash(ctx, txHash); existingChange != nil {
		logx.Infof("   ℹ️  Balance change already recorded (idempotent): tx_hash=%s change_id=%d", txHash, existingChange.ID)
		logx.Infof("✅ Deposit address balance update skipped (already processed): tx_hash=%s", msg.TxHash)
		return nil // 已处理，直接返回成功
	}

	// 3. 使用数据库事务确保余额更新和历史记录的原子性
	var markedForSweep bool
	err = p.svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		balanceRepo := p.svcCtx.WalletDepositAddressBalanceRepository.WithTx(tx)
		changeRepo := p.svcCtx.WalletDepositAddressBalanceChangeRepository.WithTx(tx)

		// 3.1 查询当前余额（如果不存在则为首次充值）
		var previousBalanceRaw, previousBalanceUSDRaw decimal.Decimal
		existing, err := balanceRepo.FindByDepositAddressAssetChain(ctx, addr.ID, assetCode, chainCode)
		if err != nil {
			// 不存在，首次充值
			logx.Infof("   📊 No existing balance record, this is first deposit")
			previousBalanceRaw = decimal.Zero
			previousBalanceUSDRaw = decimal.Zero
		} else {
			// 解析现有余额
			if v, parseErr := decimal.NewFromString(existing.BalanceRaw); parseErr == nil {
				previousBalanceRaw = v
			} else {
				logx.Errorf("⚠️  Failed to parse existing balance_raw: %s, error=%v", existing.BalanceRaw, parseErr)
				previousBalanceRaw = decimal.Zero
			}
			if v, parseErr := decimal.NewFromString(existing.BalanceUSDRaw); parseErr == nil {
				previousBalanceUSDRaw = v
			} else {
				previousBalanceUSDRaw = decimal.Zero
			}
			logx.Infof("   📊 Current balance (before deposit): %s (USD: %s cents)", previousBalanceRaw.String(), previousBalanceUSDRaw.String())
		}

		// 3.2 计算新余额 = 旧余额 + 充值金额
		newBalanceRaw := previousBalanceRaw.Add(depositAmountRaw)
		newBalance, err := accounting.RawToDecimalString(newBalanceRaw, assetPrecision)
		if err != nil {
			logx.Errorf("❌ Failed to format new balance: raw=%s precision=%d error=%v", newBalanceRaw.String(), assetPrecision, err)
			return fmt.Errorf("format new balance failed: %w", err)
		}
		logx.Infof("   📊 New balance (after deposit): %s (raw: %s)", newBalance, newBalanceRaw.String())

		// 3.3 计算 USD 价值
		var newBalanceUSD string
		var newBalanceUSDRaw decimal.Decimal
		if p.svcCtx.RedisClient != nil {
			if price, ok := logic.GetAssetPrice(ctx, p.svcCtx.RedisClient, assetCode); ok && price.IsPositive() {
				balanceDecimal, parseErr := decimal.NewFromString(newBalance)
				if parseErr == nil {
					usdValue := balanceDecimal.Mul(price)
					newBalanceUSD = usdValue.StringFixed(2)
					newBalanceUSDRaw = usdValue.Mul(decimal.NewFromInt(100))
					logx.Infof("   💵 USD value: $%s (%s cents)", newBalanceUSD, newBalanceUSDRaw.StringFixed(0))
				} else {
					logx.Errorf("⚠️  Failed to parse new balance for USD calculation: %s", newBalance)
					newBalanceUSD = "0"
					newBalanceUSDRaw = decimal.Zero
				}
			} else {
				logx.Infof("⚠️  Price not available for %s, USD value will be 0", assetCode)
				newBalanceUSD = "0"
				newBalanceUSDRaw = decimal.Zero
			}
		} else {
			logx.Infof("⚠️  Redis not available, USD value will be 0")
			newBalanceUSD = "0"
			newBalanceUSDRaw = decimal.Zero
		}

		// 3.4 更新余额表（Upsert）
		now := time.Now()
		balanceModel := &model.WalletDepositAddressBalanceModel{
			DepositAddressID:  addr.ID,
			UserID:            addr.UserID,
			AssetCode:         strings.ToUpper(strings.TrimSpace(assetCode)),
			ChainCode:         strings.ToUpper(strings.TrimSpace(chainCode)),
			Balance:           newBalance,
			BalanceRaw:        newBalanceRaw.String(),
			BalanceUSD:        newBalanceUSD,
			BalanceUSDRaw:     newBalanceUSDRaw.StringFixed(0),
			SweepThresholdRaw: "0", // TODO: 从配置表读取
			LastSyncedAt:      &now,
			SyncStatus:        "synced",
			SyncError:         nil,
		}

		if err := balanceRepo.Upsert(ctx, balanceModel); err != nil {
			logx.Errorf("❌ Failed to upsert balance: error=%v", err)
			return fmt.Errorf("upsert balance failed: %w", err)
		}

		// 3.5 查询更新后的记录以获取 needs_sweep 状态
		updated, err := balanceRepo.FindByDepositAddressAssetChain(ctx, addr.ID, assetCode, chainCode)
		if err != nil {
			logx.Errorf("⚠️  Failed to query updated balance: error=%v", err)
			markedForSweep = false
		} else {
			markedForSweep = updated.NeedsSweep == 1
			if markedForSweep {
				logx.Infof("   🔔 Address marked for sweep: balance exceeds threshold")
			}
		}

		// 3.6 记录余额变动历史
		// 注意：不在事务内部再次检查幂等性，因为已在事务外检查过

		// 计算历史 USD 值（按比例推算，避免除零错误）
		var changeAmountUSDRaw decimal.Decimal
		if newBalanceUSDRaw.IsPositive() && newBalanceRaw.IsPositive() && !newBalanceRaw.IsZero() {
			ratio := previousBalanceRaw.Div(newBalanceRaw)
			previousBalanceUSDRaw = newBalanceUSDRaw.Mul(ratio)
		}
		changeAmountUSDRaw = newBalanceUSDRaw.Sub(previousBalanceUSDRaw)

		changeReason := fmt.Sprintf("Deposit from %s", strings.TrimSpace(msg.FromAddress))
		changeModel := &model.WalletDepositAddressBalanceChangeModel{
			DepositAddressID:      addr.ID,
			UserID:                addr.UserID,
			AssetCode:             strings.ToUpper(strings.TrimSpace(assetCode)),
			ChainCode:             strings.ToUpper(strings.TrimSpace(chainCode)),
			ChangeType:            string(model.ChangeTypeDeposit),
			ChangeReason:          &changeReason,
			PreviousBalanceRaw:    previousBalanceRaw.String(),
			CurrentBalanceRaw:     newBalanceRaw.String(),
			ChangeAmountRaw:       depositAmountRaw.String(),
			PreviousBalanceUSDRaw: previousBalanceUSDRaw.String(),
			CurrentBalanceUSDRaw:  newBalanceUSDRaw.String(),
			ChangeAmountUSDRaw:    changeAmountUSDRaw.String(),
			FromAddress:           strings.TrimSpace(msg.FromAddress),
			ToAddress:             strings.TrimSpace(addr.Address),
			TxHash:                txHash,
			BlockNumber:           int64(msg.BlockNumber),
			BlockHash:             strings.TrimSpace(msg.BlockHash),
			TxIndex:               0, // TransactionConfirmMessage 中可能没有 TxIndex
			LogIndex: func() *int32 {
				if msg.LogIndex > 0 {
					idx := int32(msg.LogIndex)
					return &idx
				}
				return nil
			}(),
			BlockTimestamp:    msg.BlockTimestamp,
			ConfirmationCount: int32(msg.Confirmations),
			SweepTaskNo:       nil,
			Status:            string(model.StatusConfirmed),
			Source:            string(model.SourceChainSync),
			CreatedAt:         now,
		}

		if err := changeRepo.Create(ctx, changeModel); err != nil {
			logx.Errorf("❌ Failed to create balance change record: error=%v", err)
			return fmt.Errorf("create balance change record failed: %w", err)
		}

		logx.Infof("   ✅ Balance change recorded: tx_hash=%s deposit_addr_id=%d user_id=%d asset=%s change=+%s prev=%s curr=%s",
			txHash, addr.ID, addr.UserID, assetCode, depositAmountRaw.String(), previousBalanceRaw.String(), newBalanceRaw.String())

		return nil // 事务成功
	})

	if err != nil {
		logx.Errorf("❌ Transaction failed: error=%v", err)
		return fmt.Errorf("balance update transaction failed: %w", err)
	}

	logx.Infof("✅ Deposit address balance updated successfully: tx_hash=%s addr_id=%d user_id=%d asset=%s change=+%s sweep=%t",
		msg.TxHash, addr.ID, addr.UserID, assetCode, depositAmountRaw.String(), markedForSweep)

	return nil
}

// chainStringToChainRpcType 将 chainsync 的 chain 字符串转换为 ChainRpcType
func (p *TransactionConfirmProcessor) chainStringToChainRpcType(chain string) pb.ChainRpcType {
	switch strings.ToUpper(chain) {
	case "CHAIN_TYPE_ETHEREUM", "ETHEREUM", "ETH":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case "CHAIN_TYPE_BSC", "BSC":
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	case "CHAIN_TYPE_TRON", "TRON", "TRX":
		return pb.ChainRpcType_CHAIN_TYPE_TRON
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED
	}
}

// sendDepositSuccessEmail 发送充值成功邮件通知
func (p *TransactionConfirmProcessor) sendDepositSuccessEmail(userID int64, assetCode, chainCode, amount string, msg *mq.TransactionConfirmMessage, addr *model.WalletDepositAddressModel) {
	// 获取用户信息
	user, err := p.svcCtx.UserAccountRepository.GetByID(context.Background(), userID)
	if err != nil || user == nil {
		logx.Errorf("Failed to get user info for deposit email notification: user_id=%d error=%v", userID, err)
		return
	}

	// 准备邮件数据
	username := strings.TrimSpace(user.Nickname)
	if username == "" {
		username = user.Email
	}
	uid := fmt.Sprintf("%d", userID)

	// 格式化时间（UTC）
	depositTime := time.Now().UTC().Format("2006-01-02 15:04:05")
	if msg.BlockTimestamp > 0 {
		depositTime = time.Unix(msg.BlockTimestamp, 0).UTC().Format("2006-01-02 15:04:05")
	}

	txHash := strings.TrimSpace(msg.TxHash)
	fromAddress := strings.TrimSpace(msg.FromAddress)
	depositAddress := strings.TrimSpace(addr.Address)

	// 格式化确认数
	confirmations := fmt.Sprintf("%d", msg.Confirmations)
	requiredConfirmations := fmt.Sprintf("%d", msg.RequiredConfirmations)

	// 异步发送邮件
	err = notify.SendDepositSuccessEmailAsync(
		user.Email,
		username,
		uid,
		assetCode,
		chainCode,
		notify.FormatAmount(amount), // 格式化金额显示
		depositTime,
		txHash,
		fromAddress,
		depositAddress,
		confirmations,
		requiredConfirmations,
	)

	if err != nil {
		logx.Errorf("Failed to send deposit success email to %s: %v", user.Email, err)
	} else {
		logx.Infof("Deposit success email sent successfully to %s, asset=%s, amount=%s", user.Email, assetCode, amount)
	}
}
