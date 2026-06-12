package logic

import (
	"context"
	"fmt"
	"testing"

	tronAddress "github.com/fbsobreira/gotron-sdk/pkg/address"
	tronAPI "github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	tronCore "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/stretchr/testify/require"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

type fakeTronClient struct {
	accounts           map[string]*tronCore.Account
	resources          map[string]*tronAPI.AccountResourceMessage
	chainParams        *tronCore.ChainParameters
	estimateEnergyResp *tronAPI.EstimateEnergyMessage
	estimateEnergyErr  error
}

func (f *fakeTronClient) Stop() {}

func (f *fakeTronClient) GetAccount(addr string) (*tronCore.Account, error) {
	if f.accounts != nil {
		if acc, ok := f.accounts[addr]; ok {
			return acc, nil
		}
	}
	return nil, fmt.Errorf("account not found")
}

func (f *fakeTronClient) GetAccountResource(addr string) (*tronAPI.AccountResourceMessage, error) {
	if f.resources != nil {
		if res, ok := f.resources[addr]; ok {
			return res, nil
		}
	}
	return &tronAPI.AccountResourceMessage{}, nil
}

func (f *fakeTronClient) EstimateEnergy(ctx context.Context, trigger *tronCore.TriggerSmartContract) (*tronAPI.EstimateEnergyMessage, error) {
	if f.estimateEnergyErr != nil {
		return nil, f.estimateEnergyErr
	}
	if f.estimateEnergyResp != nil {
		return f.estimateEnergyResp, nil
	}
	return &tronAPI.EstimateEnergyMessage{EnergyRequired: 0}, nil
}

func (f *fakeTronClient) GetChainParameters(ctx context.Context) (*tronCore.ChainParameters, error) {
	return f.chainParams, nil
}

func tronBase58FromHex(t *testing.T, hex string) string {
	t.Helper()
	addr := tronAddress.HexToAddress(hex)
	require.NotNil(t, addr)
	s := addr.String()
	require.NotEmpty(t, s)
	return s
}

func TestEstimateTronFee_TRX_UnactivatedRecipient(t *testing.T) {
	from := tronBase58FromHex(t, "410000000000000000000000000000000000000001")
	to := tronBase58FromHex(t, "410000000000000000000000000000000000000002")

	fake := &fakeTronClient{
		accounts: map[string]*tronCore.Account{
			from: {Balance: 0},
		},
		resources: map[string]*tronAPI.AccountResourceMessage{
			from: {
				FreeNetLimit: 0,
				FreeNetUsed:  0,
				NetLimit:     0,
				NetUsed:      0,
				EnergyLimit:  0,
				EnergyUsed:   0,
			},
		},
		chainParams: &tronCore.ChainParameters{
			ChainParameter: []*tronCore.ChainParameters_ChainParameter{
				{Key: "getTransactionFee", Value: 1000},
				{Key: "getEnergyFee", Value: 420},
			},
		},
	}

	l := NewEstimateTronFeeLogic(context.Background(), &svc.ServiceContext{})
	l.newTronClient = func() (tronClientAPI, error) { return fake, nil }

	resp, err := l.EstimateTronFee(&pb.EstimateTronFeeReq{
		FromAddress:     from,
		ToAddress:       to,
		Amount:          "1",
		ContractAddress: "",
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	require.Equal(t, uint64(268), resp.BandwidthRequired)
	require.Equal(t, uint64(0), resp.EnergyRequired)
	require.Equal(t, "1268000", resp.EstimatedFeeSun) // 268 * 1000 + 1_000_000 activation
	require.Equal(t, "1.268000", resp.EstimatedFeeTrx)
	require.False(t, resp.ToAddressActivated)
	require.Equal(t, "1000000", resp.ActivationFeeSun)
	require.Equal(t, uint64(0), resp.FromBandwidthAvailable)
	require.Equal(t, uint64(0), resp.FromEnergyAvailable)
}

func TestEstimateTronFee_TRC20_UsesChainParamRatesAndEstimateEnergy(t *testing.T) {
	from := tronBase58FromHex(t, "410000000000000000000000000000000000000011")
	to := tronBase58FromHex(t, "410000000000000000000000000000000000000022")
	contract := tronBase58FromHex(t, "4100000000000000000000000000000000000000aa")

	fake := &fakeTronClient{
		accounts: map[string]*tronCore.Account{
			from: {Balance: 0},
			to:   {Balance: 0}, // activated
		},
		resources: map[string]*tronAPI.AccountResourceMessage{
			from: {
				FreeNetLimit: 100,
				FreeNetUsed:  0,
				NetLimit:     0,
				NetUsed:      0,
				EnergyLimit:  10000,
				EnergyUsed:   0,
			},
		},
		chainParams: &tronCore.ChainParameters{
			ChainParameter: []*tronCore.ChainParameters_ChainParameter{
				{Key: "getTransactionFee", Value: 2000},
				{Key: "getEnergyFee", Value: 500},
			},
		},
		estimateEnergyResp: &tronAPI.EstimateEnergyMessage{EnergyRequired: 32000},
	}

	l := NewEstimateTronFeeLogic(context.Background(), &svc.ServiceContext{})
	l.newTronClient = func() (tronClientAPI, error) { return fake, nil }

	resp, err := l.EstimateTronFee(&pb.EstimateTronFeeReq{
		FromAddress:     from,
		ToAddress:       to,
		Amount:          "123",
		ContractAddress: contract,
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	require.Equal(t, uint64(345), resp.BandwidthRequired)
	require.Equal(t, uint64(32000), resp.EnergyRequired)
	require.True(t, resp.ToAddressActivated)
	require.Equal(t, "0", resp.ActivationFeeSun)

	// Bandwidth shortage: (345 - 100) * 2000 = 490000
	// Energy shortage: (32000 - 10000) * 500 = 11000000
	// Total: 11490000
	require.Equal(t, "11490000", resp.EstimatedFeeSun)
	require.Equal(t, "11.490000", resp.EstimatedFeeTrx)
	require.Equal(t, uint64(100), resp.FromBandwidthAvailable)
	require.Equal(t, uint64(10000), resp.FromEnergyAvailable)
}

func TestEstimateTronFee_InvalidAddress(t *testing.T) {
	l := NewEstimateTronFeeLogic(context.Background(), &svc.ServiceContext{})
	l.newTronClient = func() (tronClientAPI, error) { return &fakeTronClient{}, nil }

	resp, err := l.EstimateTronFee(&pb.EstimateTronFeeReq{
		FromAddress: "not-a-tron-address",
		ToAddress:   "still-not-a-tron-address",
		Amount:      "1",
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.NotEmpty(t, resp.Message)
}
