package chainnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	tronAddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	tronCommon "github.com/fbsobreira/gotron-sdk/pkg/common"
	tronAPI "github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	tronCore "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"google.golang.org/protobuf/proto"

	"internalwallet/proto/pb"
)

func (c *directClient) getTronBalance(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	addr, err := requireValidTronBase58Address("address", in.Address)
	if err != nil {
		return &pb.GetBalanceResp{Success: false, Message: err.Error()}, nil
	}

	cli, _, err := c.newTron()
	if err != nil {
		return &pb.GetBalanceResp{Success: false, Message: err.Error()}, nil
	}
	defer cli.Stop()

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = reqCtx // gotron methods don't consistently take context

	acct, err := cli.GetAccount(addr)
	if err != nil {
		if isTronAccountNotFound(err) {
			return &pb.GetBalanceResp{Success: true, Message: "ok", Balance: "0", BlockNumber: 0}, nil
		}
		return &pb.GetBalanceResp{Success: false, Message: fmt.Sprintf("GetAccount failed: %v", err)}, nil
	}

	bal := big.NewInt(acct.Balance)

	// Best-effort block number
	var blockNumber uint64
	if b, bErr := cli.GetNowBlock(); bErr == nil && b != nil && b.BlockHeader != nil && b.BlockHeader.RawData != nil {
		blockNumber = uint64(b.BlockHeader.RawData.Number)
	}

	return &pb.GetBalanceResp{
		Success:     true,
		Message:     "ok",
		Balance:     bal.String(),
		BlockNumber: blockNumber,
	}, nil
}

func (c *directClient) getTronTokenBalance(ctx context.Context, in *pb.GetTokenBalanceReq) (*pb.GetTokenBalanceResp, error) {
	addr, err := requireValidTronBase58Address("address", in.Address)
	if err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: err.Error()}, nil
	}
	contract, err := requireValidTronBase58Address("token_contract", in.TokenContract)
	if err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: err.Error()}, nil
	}

	cli, _, err := c.newTron()
	if err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: err.Error()}, nil
	}
	defer cli.Stop()

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_ = reqCtx

	bal, err := cli.TRC20ContractBalance(addr, contract)
	if err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: fmt.Sprintf("TRC20ContractBalance failed: %v", err)}, nil
	}

	return &pb.GetTokenBalanceResp{
		Success: true,
		Message: "ok",
		Balance: bal.String(),
	}, nil
}

func (c *directClient) getTronTokenInfo(ctx context.Context, in *pb.GetTokenInfoReq) (*pb.GetTokenInfoResp, error) {
	contract, err := requireValidTronBase58Address("token_contract", in.TokenContract)
	if err != nil {
		return &pb.GetTokenInfoResp{Success: false, Message: err.Error()}, nil
	}

	cli, _, err := c.newTron()
	if err != nil {
		return &pb.GetTokenInfoResp{Success: false, Message: err.Error()}, nil
	}
	defer cli.Stop()

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_ = reqCtx

	symbol, _ := cli.TRC20GetSymbol(contract)
	decimalsBI, _ := cli.TRC20GetDecimals(contract)
	var decimals uint32
	if decimalsBI != nil && decimalsBI.Sign() >= 0 {
		decimals = uint32(decimalsBI.Uint64())
	}

	return &pb.GetTokenInfoResp{
		Success: true,
		Message: "ok",
		TokenInfo: &pb.TokenInfo{
			ContractAddress: contract,
			Symbol:          strings.TrimSpace(symbol),
			Decimals:        decimals,
		},
	}, nil
}

func (c *directClient) EstimateTronFee(ctx context.Context, in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
	if in == nil {
		return &pb.EstimateTronFeeResp{Success: false, Message: "request is required"}, nil
	}

	fromAddress, err := requireValidTronBase58Address("from_address", in.FromAddress)
	if err != nil {
		return &pb.EstimateTronFeeResp{Success: false, Message: err.Error()}, nil
	}
	toAddress, err := requireValidTronBase58Address("to_address", in.ToAddress)
	if err != nil {
		return &pb.EstimateTronFeeResp{Success: false, Message: err.Error()}, nil
	}
	contractAddress, err := optionalValidTronBase58Address("contract_address", in.ContractAddress)
	if err != nil {
		return &pb.EstimateTronFeeResp{Success: false, Message: err.Error()}, nil
	}

	amountStr := strings.TrimSpace(in.Amount)
	if amountStr == "" {
		return &pb.EstimateTronFeeResp{Success: false, Message: "amount is required"}, nil
	}
	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok || amount.Sign() <= 0 {
		return &pb.EstimateTronFeeResp{Success: false, Message: fmt.Sprintf("invalid amount: %s", amountStr)}, nil
	}

	tronCli, _, err := c.newTron()
	if err != nil {
		return &pb.EstimateTronFeeResp{Success: false, Message: err.Error()}, nil
	}
	defer tronCli.Stop()

	tron := &grpcTronClient{c: tronCli}

	// Recipient activation status (unactivated -> 1 TRX activation fee).
	toActivated := true
	activationFeeSun := big.NewInt(0)
	if _, err := tron.GetAccount(toAddress); err != nil {
		if isTronAccountNotFound(err) {
			toActivated = false
			activationFeeSun = big.NewInt(tronSunPerTRX)
		} else {
			return &pb.EstimateTronFeeResp{Success: false, Message: fmt.Sprintf("query to_address account: %v", err)}, nil
		}
	}

	// Sender resources (unactivated -> 0 resources)
	fromBandwidthAvailable := uint64(0)
	fromEnergyAvailable := uint64(0)
	fromActivated := true
	if _, err := tron.GetAccount(fromAddress); err != nil {
		if isTronAccountNotFound(err) {
			fromActivated = false
		} else {
			return &pb.EstimateTronFeeResp{Success: false, Message: fmt.Sprintf("query from_address account: %v", err)}, nil
		}
	}
	if fromActivated {
		res, err := tron.GetAccountResource(fromAddress)
		if err != nil {
			return &pb.EstimateTronFeeResp{Success: false, Message: fmt.Sprintf("query from_address resources: %v", err)}, nil
		}
		_, _, fromBandwidthAvailable, _, _, fromEnergyAvailable = tronResourceStats(res)
	}

	// Required resources.
	var bandwidthRequired uint64
	var energyRequired uint64
	if contractAddress == "" {
		bandwidthRequired = 268
		energyRequired = 0
	} else {
		bandwidthRequired = 345
		energyRequired = estimateTrc20TransferEnergy(ctx, tron, fromAddress, toAddress, contractAddress, amount)
	}

	bandwidthFeeSunPerUnit, energyFeeSunPerUnit := tronFeeRates(ctx, tron)

	totalFeeSun := new(big.Int).Set(activationFeeSun)
	if fromBandwidthAvailable < bandwidthRequired {
		shortage := int64(bandwidthRequired - fromBandwidthAvailable)
		totalFeeSun.Add(totalFeeSun, big.NewInt(bandwidthFeeSunPerUnit*shortage))
	}
	if energyRequired > 0 && fromEnergyAvailable < energyRequired {
		shortage := int64(energyRequired - fromEnergyAvailable)
		totalFeeSun.Add(totalFeeSun, big.NewInt(energyFeeSunPerUnit*shortage))
	}

	return &pb.EstimateTronFeeResp{
		Success:                true,
		Message:                "ok",
		BandwidthRequired:      bandwidthRequired,
		EnergyRequired:         energyRequired,
		EstimatedFeeSun:        totalFeeSun.String(),
		EstimatedFeeTrx:        formatTRXFromSun(totalFeeSun),
		EstimatedFeeUsd:        "0",
		ToAddressActivated:     toActivated,
		ActivationFeeSun:       activationFeeSun.String(),
		FromBandwidthAvailable: fromBandwidthAvailable,
		FromEnergyAvailable:    fromEnergyAvailable,
	}, nil
}

func estimateTrc20TransferEnergy(ctx context.Context, tron tronClientAPI, fromAddress, toAddress, contractAddress string, amount *big.Int) uint64 {
	const fallbackEnergy uint64 = 31_000
	if tron == nil {
		return fallbackEnergy
	}

	fromDesc, err := tronAddress.Base58ToAddress(fromAddress)
	if err != nil {
		return fallbackEnergy
	}
	toDesc, err := tronAddress.Base58ToAddress(toAddress)
	if err != nil {
		return fallbackEnergy
	}
	contractDesc, err := tronAddress.Base58ToAddress(contractAddress)
	if err != nil {
		return fallbackEnergy
	}

	// transfer(address,uint256) calldata
	ab := tronCommon.LeftPadBytes(amount.Bytes(), 32)
	zeroPad := "0000000000000000000000000000000000000000000000000000000000000000"
	toHex := toDesc.Hex()
	if len(toHex) < 4 {
		return fallbackEnergy
	}
	padIdx := len(toHex) - 4
	if padIdx < 0 || padIdx > len(zeroPad) {
		return fallbackEnergy
	}
	req := "0xa9059cbb" + zeroPad[padIdx:] + toHex[4:]
	req += tronCommon.Bytes2Hex(ab)

	dataBytes, err := tronCommon.FromHex(req)
	if err != nil {
		return fallbackEnergy
	}

	trigger := &tronCore.TriggerSmartContract{
		OwnerAddress:    fromDesc.Bytes(),
		ContractAddress: contractDesc.Bytes(),
		Data:            dataBytes,
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	energyResp, err := tron.EstimateEnergy(reqCtx, trigger)
	if err != nil || energyResp == nil || energyResp.EnergyRequired <= 0 {
		return fallbackEnergy
	}
	return uint64(energyResp.EnergyRequired)
}

func (c *directClient) GetTronFeeRates(ctx context.Context) (bandwidthFeeSunPerUnit int64, energyFeeSunPerUnit int64, err error) {
	cli, _, err := c.newTron()
	if err != nil {
		return 0, 0, err
	}
	defer cli.Stop()

	bw, en := tronFeeRates(ctx, &grpcTronClient{c: cli})
	return bw, en, nil
}

func (c *directClient) GetTronAccountResources(ctx context.Context, in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
	if in == nil {
		return &pb.GetTronAccountResourcesResp{Success: false, Message: "request is required"}, nil
	}
	addr, err := requireValidTronBase58Address("address", in.Address)
	if err != nil {
		return &pb.GetTronAccountResourcesResp{Success: false, Message: err.Error()}, nil
	}

	cli, _, err := c.newTron()
	if err != nil {
		return &pb.GetTronAccountResourcesResp{Success: false, Message: err.Error()}, nil
	}
	defer cli.Stop()

	tron := &grpcTronClient{c: cli}

	account, err := tron.GetAccount(addr)
	if err != nil {
		if isTronAccountNotFound(err) {
			return &pb.GetTronAccountResourcesResp{
				Success:            true,
				Message:            "ok",
				BandwidthTotal:     0,
				BandwidthUsed:      0,
				BandwidthAvailable: 0,
				EnergyTotal:        0,
				EnergyUsed:         0,
				EnergyAvailable:    0,
				TrxBalanceSun:      "0",
				TrxBalanceTrx:      "0",
				IsActivated:        false,
			}, nil
		}
		return &pb.GetTronAccountResourcesResp{Success: false, Message: fmt.Sprintf("GetAccount failed: %v", err)}, nil
	}

	res, err := tron.GetAccountResource(addr)
	if err != nil {
		return &pb.GetTronAccountResourcesResp{Success: false, Message: fmt.Sprintf("GetAccountResource failed: %v", err)}, nil
	}

	bwTotal, bwUsed, bwAvail, enTotal, enUsed, enAvail := tronResourceStats(res)

	balanceSun := account.Balance
	if balanceSun < 0 {
		balanceSun = 0
	}
	balanceSunBI := big.NewInt(balanceSun)

	return &pb.GetTronAccountResourcesResp{
		Success:            true,
		Message:            "ok",
		BandwidthTotal:     bwTotal,
		BandwidthUsed:      bwUsed,
		BandwidthAvailable: bwAvail,
		EnergyTotal:        enTotal,
		EnergyUsed:         enUsed,
		EnergyAvailable:    enAvail,
		TrxBalanceSun:      balanceSunBI.String(),
		TrxBalanceTrx:      formatTRXFromSun(balanceSunBI),
		IsActivated:        true,
	}, nil
}

func (c *directClient) BuildTronTransaction(ctx context.Context, in *pb.ChainRpcBuildTronTransactionReq) (*pb.ChainRpcBuildTronTransactionResp, error) {
	if in == nil {
		return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: "request is required"}, nil
	}

	fromAddress, err := requireValidTronBase58Address("from_address", in.FromAddress)
	if err != nil {
		return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: err.Error()}, nil
	}
	toAddress, err := requireValidTronBase58Address("to_address", in.ToAddress)
	if err != nil {
		return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: err.Error()}, nil
	}

	amountStr := strings.TrimSpace(in.Amount)
	if amountStr == "" {
		return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: "amount is required"}, nil
	}
	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok || amount.Sign() <= 0 {
		return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: fmt.Sprintf("invalid amount: %s", amountStr)}, nil
	}

	cli, tronCfg, err := c.newTron()
	if err != nil {
		return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: err.Error()}, nil
	}
	defer cli.Stop()

	contractAddress := strings.TrimSpace(in.ContractAddress)
	var tx *tronAPI.TransactionExtention
	if contractAddress == "" {
		tx, err = cli.Transfer(fromAddress, toAddress, amount.Int64())
		if err != nil {
			return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: fmt.Sprintf("Transfer failed: %v", err)}, nil
		}
	} else {
		contractAddress, err = requireValidTronBase58Address("contract_address", contractAddress)
		if err != nil {
			return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: err.Error()}, nil
		}
		feeLimit := tronCfg.trc20FeeLimitSun
		tx, err = cli.TRC20Send(fromAddress, toAddress, contractAddress, amount, feeLimit)
		if err != nil {
			return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: fmt.Sprintf("TRC20Send failed: %v", err)}, nil
		}
	}

	txBytes, err := proto.Marshal(tx.Transaction)
	if err != nil {
		return &pb.ChainRpcBuildTronTransactionResp{Success: false, Message: fmt.Sprintf("marshal tx: %v", err)}, nil
	}

	return &pb.ChainRpcBuildTronTransactionResp{
		Success: true,
		Message: "ok",
		RawData: hexutil.Encode(txBytes),
	}, nil
}

func (c *directClient) broadcastTron(ctx context.Context, in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error) {
	cli, _, err := c.newTron()
	if err != nil {
		return &pb.BroadcastTransactionResp{Success: false, Message: err.Error()}, nil
	}
	defer cli.Stop()

	signedTxHex := strings.TrimSpace(in.SignedTransaction)
	signedTxHex = strings.TrimPrefix(signedTxHex, "0x")
	signedTxBytes, err := hex.DecodeString(signedTxHex)
	if err != nil {
		return &pb.BroadcastTransactionResp{Success: false, Message: fmt.Sprintf("decode signed tx: %v", err)}, nil
	}

	tx := &tronCore.Transaction{}
	if err := proto.Unmarshal(signedTxBytes, tx); err != nil {
		return &pb.BroadcastTransactionResp{Success: false, Message: fmt.Sprintf("unmarshal tron tx: %v", err)}, nil
	}

	rawDataBytes, err := proto.Marshal(tx.GetRawData())
	if err != nil {
		return &pb.BroadcastTransactionResp{Success: false, Message: fmt.Sprintf("marshal raw data: %v", err)}, nil
	}
	hash := sha256.Sum256(rawDataBytes)
	txHash := hex.EncodeToString(hash[:])

	timeoutSeconds := in.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	_ = reqCtx

	result, err := cli.Broadcast(tx)
	if err != nil {
		return &pb.BroadcastTransactionResp{
			Success:       false,
			Message:       fmt.Sprintf("broadcast failed: %v", err),
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
		}, nil
	}
	if result == nil || !result.Result {
		msg := "broadcast failed"
		if result != nil && len(result.Message) > 0 {
			msg = string(result.Message)
		}
		return &pb.BroadcastTransactionResp{
			Success:       false,
			Message:       msg,
			TxHash:        txHash,
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
		}, nil
	}

	return &pb.BroadcastTransactionResp{
		Success:       true,
		Message:       "ok",
		TxHash:        txHash,
		Status:        pb.TxStatus_TX_STATUS_PENDING,
		BroadcastedAt: time.Now().Unix(),
	}, nil
}

func (c *directClient) getTronTransactionReceipt(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
	txHash := strings.TrimSpace(in.TxHash)
	if txHash == "" {
		return &pb.GetTransactionReceiptResp{Success: false, Message: "tx_hash is required"}, nil
	}

	cli, _, err := c.newTron()
	if err != nil {
		return &pb.GetTransactionReceiptResp{Success: false, Message: err.Error()}, nil
	}
	defer cli.Stop()

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_ = reqCtx

	info, err := cli.GetTransactionInfoByID(txHash)
	if err != nil || info == nil {
		// Treat as pending/not found.
		return &pb.GetTransactionReceiptResp{Success: true, Message: "receipt not found"}, nil
	}

	status := pb.TxStatus_TX_STATUS_FAILED
	if info.Result == 0 {
		status = pb.TxStatus_TX_STATUS_CONFIRMED
	}

	var gasFee string
	if info.Fee > 0 {
		gasFee = fmt.Sprintf("%d", info.Fee)
	}

	return &pb.GetTransactionReceiptResp{
		Success: true,
		Message: "ok",
		Receipt: &pb.TransactionReceipt{
			TxHash:      txHash,
			BlockNumber: fmt.Sprintf("%d", info.BlockNumber),
			GasFee:      gasFee,
			Status:      status,
		},
	}, nil
}

func (c *directClient) getTronBlockHeight(ctx context.Context) (*pb.GetBlockHeightResp, error) {
	cli, _, err := c.newTron()
	if err != nil {
		return &pb.GetBlockHeightResp{Success: false, Message: err.Error()}, nil
	}
	defer cli.Stop()

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = reqCtx

	b, err := cli.GetNowBlock()
	if err != nil || b == nil || b.BlockHeader == nil || b.BlockHeader.RawData == nil {
		return &pb.GetBlockHeightResp{Success: false, Message: fmt.Sprintf("GetNowBlock failed: %v", err)}, nil
	}
	return &pb.GetBlockHeightResp{
		Success:     true,
		Message:     "ok",
		BlockHeight: uint64(b.BlockHeader.RawData.Number),
		Timestamp:   time.Now().Unix(),
	}, nil
}
