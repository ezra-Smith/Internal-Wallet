package logic

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	tronAddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	tronCommon "github.com/fbsobreira/gotron-sdk/pkg/common"
	tronCore "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EstimateTronFeeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger

	newTronClient func() (tronClientAPI, error)
}

func NewEstimateTronFeeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EstimateTronFeeLogic {
	return &EstimateTronFeeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
		newTronClient: func() (tronClientAPI, error) {
			grpcClient, err := svcCtx.ChainMgr.GetTronClient()
			if err != nil {
				return nil, err
			}
			return &grpcTronClient{c: grpcClient}, nil
		},
	}
}

// ==================== TRON 资源/手续费 ====================
func (l *EstimateTronFeeLogic) EstimateTronFee(in *pb.EstimateTronFeeReq) (*pb.EstimateTronFeeResp, error) {
	if in == nil {
		return &pb.EstimateTronFeeResp{
			Success: false,
			Message: "request is required",
		}, nil
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

	tron, err := l.newTronClient()
	if err != nil {
		return &pb.EstimateTronFeeResp{
			Success: false,
			Message: fmt.Sprintf("failed to get TRON client: %v", err),
		}, nil
	}
	defer tron.Stop()

	// Check recipient activation status (unactivated accounts trigger a 1 TRX activation fee).
	toActivated := true
	activationFeeSun := big.NewInt(0)
	if _, err := tron.GetAccount(toAddress); err != nil {
		if isTronAccountNotFound(err) {
			toActivated = false
			activationFeeSun = big.NewInt(tronSunPerTRX)
		} else {
			return &pb.EstimateTronFeeResp{
				Success: false,
				Message: fmt.Sprintf("failed to query to_address account: %v", err),
			}, nil
		}
	}

	// Resource availability (treat unactivated from_address as 0 resources).
	var fromBandwidthAvailable uint64
	var fromEnergyAvailable uint64

	// If sender account is unactivated, it has no usable free resources.
	fromActivated := true
	if _, err := tron.GetAccount(fromAddress); err != nil {
		if isTronAccountNotFound(err) {
			fromActivated = false
		} else {
			return &pb.EstimateTronFeeResp{
				Success: false,
				Message: fmt.Sprintf("failed to query from_address account: %v", err),
			}, nil
		}
	}
	if fromActivated {
		res, err := tron.GetAccountResource(fromAddress)
		if err != nil {
			return &pb.EstimateTronFeeResp{
				Success: false,
				Message: fmt.Sprintf("failed to query from_address resources: %v", err),
			}, nil
		}
		_, _, fromBandwidthAvailable, _, _, fromEnergyAvailable = tronResourceStats(res)
	}

	// Estimate required resources.
	var bandwidthRequired uint64
	var energyRequired uint64
	if contractAddress == "" {
		// Native TRX transfer.
		bandwidthRequired = 268
		energyRequired = 0
	} else {
		// TRC20 transfer.
		bandwidthRequired = 345
		energyRequired = l.estimateTrc20TransferEnergy(tron, fromAddress, toAddress, contractAddress, amount)
	}

	bandwidthFeeSunPerUnit, energyFeeSunPerUnit := tronFeeRates(l.ctx, tron)

	totalFeeSun := new(big.Int).Set(activationFeeSun)
	if fromBandwidthAvailable < bandwidthRequired {
		shortage := int64(bandwidthRequired - fromBandwidthAvailable)
		totalFeeSun.Add(totalFeeSun, big.NewInt(bandwidthFeeSunPerUnit*shortage))
	}
	if energyRequired > 0 && fromEnergyAvailable < energyRequired {
		shortage := int64(energyRequired - fromEnergyAvailable)
		totalFeeSun.Add(totalFeeSun, big.NewInt(energyFeeSunPerUnit*shortage))
	}

	estimatedFeeUSD := calculateFeeUSD(l.svcCtx, l.Logger, pb.ChainRpcType_CHAIN_TYPE_TRON, totalFeeSun)

	return &pb.EstimateTronFeeResp{
		Success:                true,
		Message:                "ok",
		BandwidthRequired:      bandwidthRequired,
		EnergyRequired:         energyRequired,
		EstimatedFeeSun:        totalFeeSun.String(),
		EstimatedFeeTrx:        formatTRXFromSun(totalFeeSun),
		EstimatedFeeUsd:        estimatedFeeUSD,
		ToAddressActivated:     toActivated,
		ActivationFeeSun:       activationFeeSun.String(),
		FromBandwidthAvailable: fromBandwidthAvailable,
		FromEnergyAvailable:    fromEnergyAvailable,
	}, nil
}

func (l *EstimateTronFeeLogic) estimateTrc20TransferEnergy(tron tronClientAPI, fromAddress, toAddress, contractAddress string, amount *big.Int) uint64 {
	// Conservative fallback (typical TRC20 transfer ~31k-35k energy).
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

	// Build TRC20 transfer(address,uint256) calldata.
	// NOTE: TRON contracts use EVM ABI with TRON address (0x41...) represented as 20-byte address by stripping "0x41".
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

	ctx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
	defer cancel()
	energyResp, err := tron.EstimateEnergy(ctx, trigger)
	if err != nil || energyResp == nil || energyResp.EnergyRequired <= 0 {
		return fallbackEnergy
	}

	return uint64(energyResp.EnergyRequired)
}
