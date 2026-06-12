package logic

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	tronAddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	tronClient "github.com/fbsobreira/gotron-sdk/pkg/client"
	tronAPI "github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	tronCore "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
)

const (
	tronSunPerTRX          int64 = 1_000_000
	defaultBandwidthFeeSun int64 = 1000 // getTransactionFee (sun per bandwidth byte)
	defaultEnergyFeeSun    int64 = 420  // getEnergyFee (sun per energy)
)

type tronClientAPI interface {
	Stop()
	GetAccount(addr string) (*tronCore.Account, error)
	GetAccountResource(addr string) (*tronAPI.AccountResourceMessage, error)
	EstimateEnergy(ctx context.Context, trigger *tronCore.TriggerSmartContract) (*tronAPI.EstimateEnergyMessage, error)
	GetChainParameters(ctx context.Context) (*tronCore.ChainParameters, error)
}

type grpcTronClient struct {
	c *tronClient.GrpcClient
}

func (g *grpcTronClient) Stop() {
	g.c.Stop()
}

func (g *grpcTronClient) GetAccount(addr string) (*tronCore.Account, error) {
	return g.c.GetAccount(addr)
}

func (g *grpcTronClient) GetAccountResource(addr string) (*tronAPI.AccountResourceMessage, error) {
	return g.c.GetAccountResource(addr)
}

func (g *grpcTronClient) EstimateEnergy(ctx context.Context, trigger *tronCore.TriggerSmartContract) (*tronAPI.EstimateEnergyMessage, error) {
	return g.c.Client.EstimateEnergy(ctx, trigger)
}

func (g *grpcTronClient) GetChainParameters(ctx context.Context) (*tronCore.ChainParameters, error) {
	return g.c.Client.GetChainParameters(ctx, new(tronAPI.EmptyMessage))
}

func requireValidTronBase58Address(field string, addr string) (string, error) {
	clean := strings.TrimSpace(addr)
	if clean == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if _, err := tronAddress.Base58ToAddress(clean); err != nil {
		return "", fmt.Errorf("invalid %s: %v", field, err)
	}
	return clean, nil
}

func optionalValidTronBase58Address(field string, addr string) (string, error) {
	clean := strings.TrimSpace(addr)
	if clean == "" {
		return "", nil
	}
	if _, err := tronAddress.Base58ToAddress(clean); err != nil {
		return "", fmt.Errorf("invalid %s: %v", field, err)
	}
	return clean, nil
}

func isTronAccountNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "account not found")
}

func tronFeeRates(ctx context.Context, tron tronClientAPI) (bandwidthFeeSunPerUnit int64, energyFeeSunPerUnit int64) {
	bandwidthFeeSunPerUnit = defaultBandwidthFeeSun
	energyFeeSunPerUnit = defaultEnergyFeeSun
	if tron == nil {
		return bandwidthFeeSunPerUnit, energyFeeSunPerUnit
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	params, err := tron.GetChainParameters(reqCtx)
	if err != nil || params == nil {
		return bandwidthFeeSunPerUnit, energyFeeSunPerUnit
	}

	for _, p := range params.GetChainParameter() {
		key := strings.TrimSpace(p.GetKey())
		switch {
		case strings.EqualFold(key, "getTransactionFee"):
			if v := p.GetValue(); v > 0 {
				bandwidthFeeSunPerUnit = v
			}
		case strings.EqualFold(key, "getEnergyFee"):
			if v := p.GetValue(); v > 0 {
				energyFeeSunPerUnit = v
			}
		}
	}

	return bandwidthFeeSunPerUnit, energyFeeSunPerUnit
}

func tronResourceStats(res *tronAPI.AccountResourceMessage) (bandwidthTotal, bandwidthUsed, bandwidthAvailable, energyTotal, energyUsed, energyAvailable uint64) {
	if res == nil {
		return 0, 0, 0, 0, 0, 0
	}

	bwTotal := res.GetFreeNetLimit() + res.GetNetLimit()
	bwUsed := res.GetFreeNetUsed() + res.GetNetUsed()
	bwAvail := bwTotal - bwUsed
	if bwTotal < 0 {
		bwTotal = 0
	}
	if bwUsed < 0 {
		bwUsed = 0
	}
	if bwAvail < 0 {
		bwAvail = 0
	}

	enTotal := res.GetEnergyLimit()
	enUsed := res.GetEnergyUsed()
	enAvail := enTotal - enUsed
	if enTotal < 0 {
		enTotal = 0
	}
	if enUsed < 0 {
		enUsed = 0
	}
	if enAvail < 0 {
		enAvail = 0
	}

	return uint64(bwTotal), uint64(bwUsed), uint64(bwAvail), uint64(enTotal), uint64(enUsed), uint64(enAvail)
}

func formatTRXFromSun(sun *big.Int) string {
	if sun == nil || sun.Sign() == 0 {
		return "0"
	}
	f := new(big.Float).SetInt(sun)
	f.Quo(f, big.NewFloat(float64(tronSunPerTRX)))
	return f.Text('f', 6)
}
