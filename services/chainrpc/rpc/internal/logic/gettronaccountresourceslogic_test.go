package logic

import (
	"context"
	"testing"

	tronAPI "github.com/fbsobreira/gotron-sdk/pkg/proto/api"
	tronCore "github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/stretchr/testify/require"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"
)

func TestGetTronAccountResources_Activated(t *testing.T) {
	addr := tronBase58FromHex(t, "4100000000000000000000000000000000000000ff")

	fake := &fakeTronClient{
		accounts: map[string]*tronCore.Account{
			addr: {Balance: 12345678},
		},
		resources: map[string]*tronAPI.AccountResourceMessage{
			addr: {
				FreeNetLimit: 5000,
				FreeNetUsed:  1000,
				NetLimit:     1000,
				NetUsed:      200,
				EnergyLimit:  20000,
				EnergyUsed:   5000,
			},
		},
	}

	l := NewGetTronAccountResourcesLogic(context.Background(), &svc.ServiceContext{})
	l.newTronClient = func() (tronClientAPI, error) { return fake, nil }

	resp, err := l.GetTronAccountResources(&pb.GetTronAccountResourcesReq{Address: addr})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.True(t, resp.IsActivated)

	require.Equal(t, uint64(6000), resp.BandwidthTotal)
	require.Equal(t, uint64(1200), resp.BandwidthUsed)
	require.Equal(t, uint64(4800), resp.BandwidthAvailable)

	require.Equal(t, uint64(20000), resp.EnergyTotal)
	require.Equal(t, uint64(5000), resp.EnergyUsed)
	require.Equal(t, uint64(15000), resp.EnergyAvailable)

	require.Equal(t, "12345678", resp.TrxBalanceSun)
	require.Equal(t, "12.345678", resp.TrxBalanceTrx)
}

func TestGetTronAccountResources_Unactivated(t *testing.T) {
	addr := tronBase58FromHex(t, "4100000000000000000000000000000000000000ee")

	fake := &fakeTronClient{
		accounts:  map[string]*tronCore.Account{},
		resources: map[string]*tronAPI.AccountResourceMessage{},
	}

	l := NewGetTronAccountResourcesLogic(context.Background(), &svc.ServiceContext{})
	l.newTronClient = func() (tronClientAPI, error) { return fake, nil }

	resp, err := l.GetTronAccountResources(&pb.GetTronAccountResourcesReq{Address: addr})
	require.NoError(t, err)
	require.True(t, resp.Success)
	require.False(t, resp.IsActivated)
	require.Equal(t, uint64(0), resp.BandwidthTotal)
	require.Equal(t, uint64(0), resp.EnergyTotal)
	require.Equal(t, "0", resp.TrxBalanceSun)
	require.Equal(t, "0", resp.TrxBalanceTrx)
}

func TestGetTronAccountResources_InvalidAddress(t *testing.T) {
	l := NewGetTronAccountResourcesLogic(context.Background(), &svc.ServiceContext{})
	l.newTronClient = func() (tronClientAPI, error) { return &fakeTronClient{}, nil }

	resp, err := l.GetTronAccountResources(&pb.GetTronAccountResourcesReq{Address: "invalid"})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.NotEmpty(t, resp.Message)
}
