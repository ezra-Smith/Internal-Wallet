package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	core "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	commonutils "internalwallet/common/utils"

	tronAddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/errx"
	"internalwallet/services/business/rpc/internal/model"
	"internalwallet/services/business/rpc/internal/svc"
)

type BroadcastWeb3TransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBroadcastWeb3TransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BroadcastWeb3TransactionLogic {
	return &BroadcastWeb3TransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BroadcastWeb3Transaction 广播 Web3 交易
func (l *BroadcastWeb3TransactionLogic) BroadcastWeb3Transaction(in *pb.BroadcastWeb3TransactionReq) (*pb.BroadcastWeb3TransactionResp, error) {
	if in == nil {
		return nil, errx.InvalidParam("request is required")
	}

	// 验证必填参数
	deviceID := strings.TrimSpace(in.DeviceId)
	signedTxHex := strings.TrimSpace(in.SignedTransaction)
	networkParam := strings.TrimSpace(in.Network)
	fromAddress := strings.TrimSpace(in.FromAddress)
	swapID := strings.TrimSpace(in.SwapId)

	// 标准化网络字段为链代码格式（ETH/BSC/TRON），确保与数据库和前端查询一致
	network := normalizeNetworkToChainCode(networkParam)

	if deviceID == "" {
		return nil, errx.Web3DeviceIDRequired()
	}
	if signedTxHex == "" {
		return nil, errx.Web3SignedTxRequired()
	}
	if network == "" {
		return nil, errx.Web3NetworkRequired()
	}
	if fromAddress == "" {
		return nil, errx.Web3FromAddressRequired()
	}

	// Swap service only supports EVM chains (ETH/BSC). For TRON, ignore swap_id to avoid polluting swap tables.
	shouldUpdateSwapRecord := swapID != "" && network != "TRON"

	// 1. 验证地址是否存在（不验证用户归属，因为地址可以通过助记词导入到不同设备）
	// 注意：余额记录是地址级别的，不绑定到特定用户/设备
	web3UserAddress, err := l.svcCtx.Web3UserAddressRepository.FindByAddress(l.ctx, fromAddress)
	if err != nil || web3UserAddress == nil {
		// 如果地址不存在，仍然允许广播交易（可能是新导入的地址）
		l.Infof("地址未在系统中注册: address=%s, network=%s (允许继续广播交易)", fromAddress, network)
	} else {
		// 检查地址是否启用
		if !web3UserAddress.Enabled {
			return nil, errx.Web3AddressDisabled()
		}

		// 检查地址是否在黑名单中
		if web3UserAddress.IsBlacklisted {
			return nil, errx.Web3AddressBlacklisted()
		}
	}

	// 2.5 检查账户激活状态和 Gas 余额（防止广播失败后状态卡在 pending）
	if err := l.checkAccountAndGas(network, fromAddress); err != nil {
		return nil, err
	}

	// 3. 解析交易以获取交易信息（用于记录）
	txInfo, err := l.parseTransaction(signedTxHex, network)
	if err != nil {
		l.Errorf("解析交易失败: error=%v", err)
		return nil, errx.Web3ParseTxFailed(err.Error())
	}

	// 验证解析出的 from_address 是否与请求中的一致
	if !strings.EqualFold(txInfo.FromAddress, fromAddress) {
		l.Errorf("交易中的发送方地址与请求不一致: tx_from=%s, req_from=%s", txInfo.FromAddress, fromAddress)
		return nil, errx.Web3FromAddressMismatch()
	}

	// 4. 记录交易到 web3_transactions 表（状态：pending）
	now := time.Now()
	txType := strings.TrimSpace(in.TxType)
	if txType == "" {
		txType = model.Web3TxTypeSend // 默认发送
	}

	assetCode := strings.TrimSpace(in.AssetCode)
	amount := strings.TrimSpace(in.Amount)
	toAddress := strings.TrimSpace(in.ToAddress)

	// 如果未提供 asset_code 和 amount，尝试从交易中提取
	if assetCode == "" || amount == "" {
		// 从交易信息中获取
		if txInfo.AssetCode != "" {
			assetCode = txInfo.AssetCode
		}
		if txInfo.Amount != "" {
			amount = txInfo.Amount
		}
	}

	// 初始化区块时间为当前时间（pending状态下的占位符）
	// 后续交易确认后会更新为实际的区块时间
	blockTime := now

	// 获取 chain_id（如果地址存在则使用地址的 chain_id，否则使用默认值）
	chainID := int64(0)
	if web3UserAddress != nil {
		chainID = web3UserAddress.ChainID
	}

	// 保存“广播占位”记录到 web3_balance_changes（event_index = -1）
	web3Change := &model.Web3BalanceChangeModel{
		Web3UserID:    0, // 地址级别记录，不绑定到特定用户/设备
		UserAddress:   fromAddress,
		ChainCode:     network,
		ChainID:       chainID,
		TxHash:        txInfo.TxHash,
		EventIndex:    model.Web3BalanceChangeBroadcastEventIndex,
		BlockNumber:   nil,
		BlockTime:     &blockTime, // pending 占位符，确认后由 chainsync 写真实区块时间
		TxType:        txType,
		Direction:     model.Web3TxDirectionOut,
		AssetCode:     assetCode,
		Amount:        amount,
		AmountRaw:     nil,
		TokenAddress:  nil,
		TokenDecimals: nil,
		FromAddress:   stringPtr(fromAddress),
		ToAddress:     stringPtr(toAddress),
		Status:        model.Web3TxStatusPending,
		Confirmations: 0,
		// IMPORTANT: placeholder fee must be empty. Only write `fee` when confirmed and chain data is available.
		Fee:      nil,
		FeeAsset: nil,
		RawData:  nil,
	}

	if err := l.svcCtx.Web3BalanceChangeRepository.Upsert(l.ctx, web3Change); err != nil {
		l.Errorf("记录Web3交易失败(web3_balance_changes): tx_hash=%s, error=%v", txInfo.TxHash, err)
		return nil, errx.Web3RecordTxFailed(err.Error())
	}

	// 5. 调用 ChainRPC 广播交易
	if l.svcCtx.ChainRpc == nil {
		l.Errorf("ChainRpc客户端未初始化")
		return nil, errx.Web3ChainServiceUnavailable()
	}

	// 转换网络名称到 ChainRpcType
	chainType := l.mapNetworkToChainType(network)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		return nil, errx.Web3UnsupportedNetwork(network)
	}

	waitForReceipt := in.WaitForReceipt
	timeoutSeconds := in.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}

	broadcastReq := &pb.BroadcastTransactionReq{
		Chain:             chainType,
		SignedTransaction: signedTxHex,
		RequestId:         fmt.Sprintf("web3_%s_%d", deviceID, now.Unix()),
		WaitForReceipt:    waitForReceipt,
		TimeoutSeconds:    timeoutSeconds,
	}

	broadcastResp, err := l.svcCtx.ChainRpc.BroadcastTransaction(l.ctx, broadcastReq)
	if err != nil || broadcastResp == nil || !broadcastResp.Success {
		errorMsg := ""
		if broadcastResp != nil && broadcastResp.Message != "" {
			errorMsg = broadcastResp.Message
		}
		if err != nil {
			if errorMsg != "" {
				errorMsg = fmt.Sprintf("%s: %v", errorMsg, err)
			} else {
				errorMsg = err.Error()
			}
		}

		// 更新交易状态为失败
		web3Change.Status = model.Web3TxStatusFailed
		if updateErr := l.svcCtx.Web3BalanceChangeRepository.Upsert(l.ctx, web3Change); updateErr != nil {
			l.Errorf("更新交易状态为失败时出错: tx_hash=%s, error=%v", txInfo.TxHash, updateErr)
		}

		// Best-effort: update swap_svc_transactions (do not affect broadcast failure handling).
		if shouldUpdateSwapRecord && l.svcCtx.SwapRpc != nil {
			ctxUpdate, cancel := context.WithTimeout(l.ctx, 2*time.Second)
			_, updErr := l.svcCtx.SwapRpc.UpdateSwapBroadcastResult(ctxUpdate, &pb.UpdateSwapBroadcastResultRequest{
				SwapId:           swapID,
				ChainId:          txInfo.ChainID,
				TxHash:           txInfo.TxHash,
				BroadcastedAt:    0,
				BroadcastSuccess: false,
				ErrorCode:        "BROADCAST_FAILED",
				ErrorMessage:     errorMsg,
			})
			cancel()
			if updErr != nil {
				l.Errorw("Failed to update swap record after broadcast failure",
					logx.Field("error", updErr),
					logx.Field("swap_id", swapID),
					logx.Field("tx_hash", txInfo.TxHash),
				)
			}
		}

		l.Errorf("广播交易失败: tx_hash=%s, error=%v", txInfo.TxHash, err)

		// 解析错误消息，识别特定的错误类型并返回对应的国际化错误码
		return nil, l.parseBroadcastError(errorMsg, network)
	}

	// 6. 更新交易状态和 tx_hash（如果 ChainRPC 返回的 tx_hash 不同）
	finalTxHash := broadcastResp.TxHash
	if finalTxHash == "" {
		finalTxHash = txInfo.TxHash
	}

	// 如果 tx_hash 发生变化，更新记录
	if finalTxHash != txInfo.TxHash {
		l.Logger.Infof("交易哈希不一致: expected=%s, got=%s", txInfo.TxHash, finalTxHash)
		// 可能需要更新或创建新记录，这里简化处理，使用 ChainRPC 返回的
	}

	// 更新交易状态
	status := model.Web3TxStatusPending
	if broadcastResp.Status == pb.TxStatus_TX_STATUS_CONFIRMED {
		status = model.Web3TxStatusConfirmed
	} else if broadcastResp.Status == pb.TxStatus_TX_STATUS_FAILED {
		status = model.Web3TxStatusFailed
	}

	// 同步更新“广播占位”记录的状态（event_index = -1）
	web3Change.TxHash = finalTxHash
	web3Change.Status = status

	// IMPORTANT:
	// Placeholder fee is estimated at broadcast time (gas_limit * gas_price). Once the tx is confirmed,
	// overwrite it with the on-chain actual fee from receipt (gas_used * effective_gas_price).
	// This avoids showing stale/overestimated fees after confirmation.
	if waitForReceipt && status == model.Web3TxStatusConfirmed && l.svcCtx.ChainRpc != nil {
		if r, err := l.svcCtx.ChainRpc.GetTransactionReceipt(l.ctx, &pb.GetTransactionReceiptReq{
			Chain:  chainType,
			TxHash: finalTxHash,
		}); err == nil && r != nil && r.Success && r.Receipt != nil {
			feePtr, feeAssetPtr := commonutils.FormatWeb3Fee(network, r.Receipt.GasFee, r.Receipt.GasUsed, "")
			if feePtr != nil {
				v := strings.TrimSpace(*feePtr)
				if v != "" {
					web3Change.Fee = &v
				}
			}
			if feeAssetPtr != nil {
				v := strings.TrimSpace(*feeAssetPtr)
				if v != "" {
					web3Change.FeeAsset = &v
				}
			}
		}
	}
	if updateErr := l.svcCtx.Web3BalanceChangeRepository.Upsert(l.ctx, web3Change); updateErr != nil {
		l.Errorf("更新交易状态时出错: tx_hash=%s, status=%s, error=%v", finalTxHash, status, updateErr)
	}

	var swapRecordOk *bool
	if shouldUpdateSwapRecord {
		v := false
		swapRecordOk = &v
		if l.svcCtx.SwapRpc == nil {
			l.Errorw("Swap service not available, cannot update swap record",
				logx.Field("swap_id", swapID),
				logx.Field("tx_hash", finalTxHash),
			)
		} else {
			ctxUpdate, cancel := context.WithTimeout(l.ctx, 2*time.Second)
			resp, err := l.svcCtx.SwapRpc.UpdateSwapBroadcastResult(ctxUpdate, &pb.UpdateSwapBroadcastResultRequest{
				SwapId:           swapID,
				ChainId:          txInfo.ChainID,
				TxHash:           finalTxHash,
				BroadcastedAt:    broadcastResp.BroadcastedAt,
				BroadcastSuccess: true,
				ErrorCode:        "",
				ErrorMessage:     "",
			})
			cancel()
			if err != nil {
				l.Errorw("Failed to update swap record after broadcast success",
					logx.Field("error", err),
					logx.Field("swap_id", swapID),
					logx.Field("tx_hash", finalTxHash),
				)
			} else if resp == nil || !resp.Success {
				msg := ""
				if resp != nil {
					msg = resp.Message
				}
				l.Errorw("Swap service returned failure when updating swap record after broadcast success",
					logx.Field("swap_id", swapID),
					logx.Field("tx_hash", finalTxHash),
					logx.Field("message", msg),
				)
			} else {
				*swapRecordOk = true
			}
		}
	}

	return &pb.BroadcastWeb3TransactionResp{
		Success:       true,
		Message:       "ok",
		TxHash:        finalTxHash,
		Status:        status,
		BroadcastedAt: broadcastResp.BroadcastedAt,
		BlockNumber:   broadcastResp.BlockNumber,
		SwapRecordOk:  swapRecordOk,
	}, nil
}

// TransactionInfo 交易信息
type TransactionInfo struct {
	ChainID     int64
	TxHash      string
	FromAddress string
	ToAddress   string
	Amount      string
	AssetCode   string
	Fee         string
	FeeAsset    string
}

// parseTransaction 解析交易以获取交易信息
func (l *BroadcastWeb3TransactionLogic) parseTransaction(signedTxHex string, network string) (*TransactionInfo, error) {
	signedTxHex = strings.TrimSpace(signedTxHex)
	signedTxHex = strings.TrimPrefix(signedTxHex, "0x")

	signedTxBytes, err := hex.DecodeString(signedTxHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %v", err)
	}

	info := &TransactionInfo{}

	// 根据网络类型解析
	networkLower := strings.ToLower(network)
	if networkLower == "tron" || networkLower == "trx" {
		// TRON 交易
		tx := &core.Transaction{}
		if err := proto.Unmarshal(signedTxBytes, tx); err != nil {
			return nil, fmt.Errorf("failed to unmarshal TRON transaction: %v", err)
		}

		// 计算交易哈希（TRON 使用 SHA256）
		rawDataBytes, err := proto.Marshal(tx.GetRawData())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal TRON raw data: %v", err)
		}
		hashBytes := sha256.Sum256(rawDataBytes)
		info.TxHash = hex.EncodeToString(hashBytes[:])

		contracts := tx.GetRawData().GetContract()
		if len(contracts) > 0 {
			contract := contracts[0]
			if contract != nil && contract.GetParameter() != nil {
				switch contract.GetType() {
				case core.Transaction_Contract_TransferContract:
					transfer := &core.TransferContract{}
					if err := unmarshalAnyValue(contract.GetParameter(), transfer); err != nil {
						return nil, fmt.Errorf("failed to parse TRON transfer contract: %v", err)
					}
					info.FromAddress = tronBase58FromBytes(transfer.GetOwnerAddress())
					info.ToAddress = tronBase58FromBytes(transfer.GetToAddress())
					info.Amount = fmt.Sprintf("%d", transfer.GetAmount())
					info.AssetCode = "TRX"
				case core.Transaction_Contract_TriggerSmartContract:
					trigger := &core.TriggerSmartContract{}
					if err := unmarshalAnyValue(contract.GetParameter(), trigger); err != nil {
						return nil, fmt.Errorf("failed to parse TRON trigger contract: %v", err)
					}
					info.FromAddress = tronBase58FromBytes(trigger.GetOwnerAddress())
					info.ToAddress = tronBase58FromBytes(trigger.GetContractAddress())
					info.FeeAsset = "TRX"

					toAddr, amount, ok := parseTrc20TransferData(trigger.GetData())
					if ok {
						info.ToAddress = toAddr
						info.Amount = amount
					}
				}
			}
		}

		if info.FeeAsset == "" {
			info.FeeAsset = "TRX"
		}
	} else {
		// EVM 兼容链（ETH/BSC）
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(signedTxBytes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ETH transaction: %v", err)
		}

		if tx.ChainId() != nil {
			info.ChainID = tx.ChainId().Int64()
		}
		info.TxHash = tx.Hash().Hex()

		// 获取发送方地址
		signer := types.LatestSignerForChainID(tx.ChainId())
		from, err := types.Sender(signer, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to get sender: %v", err)
		}
		info.FromAddress = from.Hex()

		// 获取接收方地址
		if tx.To() != nil {
			info.ToAddress = tx.To().Hex()
		}

		// 获取金额（wei 转 eth）
		if tx.Value() != nil && tx.Value().Cmp(big.NewInt(0)) > 0 {
			ethValue := new(big.Float).Quo(new(big.Float).SetInt(tx.Value()), big.NewFloat(1e18))
			info.Amount = ethValue.Text('f', 18)
			info.AssetCode = "ETH" // 默认，可能需要在调用时指定
		}

		// 计算手续费
		gasPrice := tx.GasPrice()
		gasLimit := tx.Gas()
		if gasPrice != nil && gasLimit > 0 {
			fee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
			feeEth := new(big.Float).Quo(new(big.Float).SetInt(fee), big.NewFloat(1e18))
			info.Fee = feeEth.Text('f', 18)
			if networkLower == "bsc" {
				info.FeeAsset = "BNB"
			} else {
				info.FeeAsset = "ETH"
			}
		}
	}

	return info, nil
}

func unmarshalAnyValue(anyMsg *anypb.Any, target proto.Message) error {
	if anyMsg == nil {
		return fmt.Errorf("empty contract parameter")
	}
	if len(anyMsg.Value) == 0 {
		return fmt.Errorf("empty contract parameter value")
	}
	return proto.Unmarshal(anyMsg.Value, target)
}

func tronBase58FromBytes(addr []byte) string {
	if len(addr) == 0 {
		return ""
	}
	if len(addr) == tronAddress.AddressLength {
		return tronAddress.Address(addr).String()
	}
	if len(addr) == tronAddress.AddressLength-1 {
		full := make([]byte, 0, tronAddress.AddressLength)
		full = append(full, tronAddress.TronBytePrefix)
		full = append(full, addr...)
		return tronAddress.Address(full).String()
	}
	return ""
}

func parseTrc20TransferData(data []byte) (toBase58 string, amount string, ok bool) {
	if len(data) < 4+32+32 {
		return "", "", false
	}
	// function selector: a9059cbb
	if !bytesEqual(data[0:4], []byte{0xa9, 0x05, 0x9c, 0xbb}) {
		return "", "", false
	}
	toBytes := data[4+12 : 4+32]
	amtBytes := data[4+32 : 4+64]

	toBase58 = tronBase58FromBytes(append([]byte{tronAddress.TronBytePrefix}, toBytes...))
	amt := new(big.Int).SetBytes(amtBytes)
	amount = amt.String()
	return toBase58, amount, true
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// mapNetworkToChainType 将网络名称映射到 ChainRpcType
func (l *BroadcastWeb3TransactionLogic) mapNetworkToChainType(network string) pb.ChainRpcType {
	networkLower := strings.ToLower(network)
	switch networkLower {
	case "ethereum", "eth":
		return pb.ChainRpcType_CHAIN_TYPE_ETHEREUM
	case "bsc", "binance", "binance smart chain":
		return pb.ChainRpcType_CHAIN_TYPE_BSC
	case "tron", "trx":
		return pb.ChainRpcType_CHAIN_TYPE_TRON
	default:
		return pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED
	}
}

// normalizeNetworkToChainCode 将网络字段标准化为链代码格式（ETH/BSC/TRON）
// 确保与前端查询和数据库存储一致
func normalizeNetworkToChainCode(network string) string {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "ethereum", "eth", "chain_type_ethereum":
		return "ETH"
	case "bsc", "binance", "binance smart chain", "chain_type_bsc":
		return "BSC"
	case "tron", "trx", "chain_type_tron":
		return "TRON"
	default:
		// 如果已经是标准格式，直接返回
		return strings.ToUpper(strings.TrimSpace(network))
	}
}

// checkAccountAndGas 检查账户激活状态和 Gas 余额
// 在广播交易前进行预检查，防止广播失败后状态卡在 pending
func (l *BroadcastWeb3TransactionLogic) checkAccountAndGas(network string, fromAddress string) error {
	if l.svcCtx.ChainRpc == nil {
		// ChainRpc 未配置，跳过检查
		return nil
	}

	chainType := l.mapNetworkToChainType(network)
	if chainType == pb.ChainRpcType_CHAIN_TYPE_UNSPECIFIED {
		// 不支持的链，跳过检查
		return nil
	}

	// 获取原生代币余额
	ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	balanceResp, err := l.svcCtx.ChainRpc.GetBalance(ctx, &pb.GetBalanceReq{
		Chain:   chainType,
		Address: fromAddress,
	})

	if err != nil {
		// 检查是否是"账户未找到"错误（TRON 网络账户未激活）
		errMsg := err.Error()
		if strings.Contains(errMsg, "account not found") ||
			strings.Contains(errMsg, "Account not found") ||
			strings.Contains(errMsg, "account does not exist") {
			l.Errorf("账户未激活，无法发起交易: network=%s, address=%s, error=%v", network, fromAddress, err)
			return errx.Web3AccountNotActivated()
		}
		// 其他错误，记录日志但不阻止交易（可能是网络问题）
		l.Errorf("检查账户余额失败，继续尝试广播: network=%s, address=%s, error=%v", network, fromAddress, err)
		return nil
	}

	if balanceResp == nil || !balanceResp.Success {
		msg := ""
		if balanceResp != nil {
			msg = balanceResp.Message
		}
		// 检查响应消息中是否包含账户未找到
		if strings.Contains(msg, "account not found") ||
			strings.Contains(msg, "Account not found") ||
			strings.Contains(msg, "account does not exist") {
			l.Errorf("账户未激活，无法发起交易: network=%s, address=%s, message=%s", network, fromAddress, msg)
			return errx.Web3AccountNotActivated()
		}
		// 其他失败，记录日志但不阻止
		l.Errorf("获取账户余额响应失败，继续尝试广播: network=%s, address=%s, message=%s", network, fromAddress, msg)
		return nil
	}

	// 检查原生代币余额（用于 Gas）
	nativeBalance := balanceResp.Balance
	if nativeBalance == "" {
		nativeBalance = "0"
	}

	// 解析余额
	balance, ok := new(big.Float).SetString(nativeBalance)
	if !ok {
		balance = new(big.Float).SetFloat64(0)
	}

	// 根据不同链设置最低 Gas 要求
	var minGas *big.Float
	var gasAsset string
	switch network {
	case "TRON":
		// TRON: 至少需要一点 TRX 来支付带宽/能量费用
		// 对于 TRC20 转账，通常需要约 10-30 TRX
		minGas = new(big.Float).SetFloat64(0.1) // 最低 0.1 TRX
		gasAsset = "TRX"
	case "ETH":
		// ETH: ERC20 转账大约需要 0.002-0.01 ETH
		minGas = new(big.Float).SetFloat64(0.0001) // 最低 0.0001 ETH
		gasAsset = "ETH"
	case "BSC":
		// BSC: BEP20 转账大约需要 0.0005-0.002 BNB
		minGas = new(big.Float).SetFloat64(0.0001) // 最低 0.0001 BNB
		gasAsset = "BNB"
	default:
		// 未知链，跳过 Gas 检查
		return nil
	}

	// 如果余额为 0 或小于最低要求，报错
	if balance.Cmp(new(big.Float).SetFloat64(0)) <= 0 {
		l.Errorf("账户原生代币余额为 0，可能未激活或 Gas 不足: network=%s, address=%s, balance=%s %s",
			network, fromAddress, nativeBalance, gasAsset)
		// 如果余额完全为 0，很可能是账户未激活
		return errx.Web3AccountNotActivated()
	}

	if balance.Cmp(minGas) < 0 {
		minGasStr := minGas.Text('f', 6)
		l.Errorf("Gas 不足: network=%s, address=%s, required=%s %s, available=%s %s",
			network, fromAddress, minGasStr, gasAsset, nativeBalance, gasAsset)
		return errx.Web3InsufficientGas(network, minGasStr+" "+gasAsset, nativeBalance+" "+gasAsset)
	}

	l.Infof("账户预检查通过: network=%s, address=%s, balance=%s %s", network, fromAddress, nativeBalance, gasAsset)
	return nil
}

// parseBroadcastError 解析广播交易错误消息，识别特定的错误类型并返回对应的国际化错误
func (l *BroadcastWeb3TransactionLogic) parseBroadcastError(errorMsg string, network string) error {
	if errorMsg == "" {
		return errx.Web3BroadcastFailed("")
	}

	lowerMsg := strings.ToLower(errorMsg)

	// TRON 网络特定错误
	if network == "TRON" {
		// 账户资源不足（带宽或能量）
		if strings.Contains(lowerMsg, "account resource insufficient") ||
			strings.Contains(lowerMsg, "resource insufficient") ||
			strings.Contains(lowerMsg, "bandwidth") ||
			strings.Contains(lowerMsg, "energy") {
			return errx.Web3AccountResourceInsufficient()
		}

		// 账户未激活
		if strings.Contains(lowerMsg, "account not found") ||
			strings.Contains(lowerMsg, "account not exist") ||
			strings.Contains(lowerMsg, "account does not exist") {
			return errx.Web3AccountNotActivated()
		}

		// 余额不足
		if strings.Contains(lowerMsg, "insufficient balance") {
			return errx.Web3InsufficientGas(network, "", "")
		}
	}

	// EVM 链（ETH/BSC）特定错误
	if network == "ETH" || network == "BSC" {
		// Gas 不足
		if strings.Contains(lowerMsg, "insufficient funds") ||
			strings.Contains(lowerMsg, "gas") ||
			strings.Contains(lowerMsg, "insufficient balance") {
			return errx.Web3InsufficientGas(network, "", "")
		}
	}

	// 通用错误：返回原始错误消息（但会通过国际化处理）
	return errx.Web3BroadcastFailed(errorMsg)
}
