package scheduler

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"internalwallet/proto/pb"
	"internalwallet/services/consolidation/rpc/internal/config"
	"internalwallet/services/consolidation/rpc/internal/svc"
)

func TestConsolidationScheduler_GetTargetAddress_TargetAddressesMode(t *testing.T) {
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				ToAddressMode: "target_addresses",
				TargetAddresses: map[string]map[string]string{
					"ETH": {
						"USDT": "0x000000000000000000000000000000000000dEaD",
					},
				},
			},
		},
	}
	s := NewConsolidationScheduler(svcCtx)
	addr := s.getTargetAddress(context.Background(), "eth", "usdt")
	require.Equal(t, "0x000000000000000000000000000000000000dEaD", addr)
}

func TestConsolidationScheduler_GetTargetAddress_SystemHotWalletMode_CachedPerChain(t *testing.T) {
	var calls int32
	signer := &stubSigner{
		getCompanyWalletFn: func(ctx context.Context, in *pb.GetCompanyWalletRequest, opts ...grpc.CallOption) (*pb.GetCompanyWalletResponse, error) {
			atomic.AddInt32(&calls, 1)
			return &pb.GetCompanyWalletResponse{
				Code: 0,
				Wallet: &pb.CompanyWalletInfo{
					Chain:   in.GetChain(),
					Address: "0x000000000000000000000000000000000000bEEF",
				},
			}, nil
		},
	}
	svcCtx := &svc.ServiceContext{
		Config: config.Config{
			Consolidation: config.ConsolidationConfig{
				ToAddressMode:              "system_hot_wallet",
				SystemHotWalletAddressType: "hot_primary",
				SystemHotWalletTemperature: 1,
			},
		},
		SignerRpc: signer,
	}
	s := NewConsolidationScheduler(svcCtx)
	addr1 := s.getTargetAddress(context.Background(), "eth", "usdt")
	addr2 := s.getTargetAddress(context.Background(), "ETH", "ETH")
	require.Equal(t, "0x000000000000000000000000000000000000bEEF", addr1)
	require.Equal(t, "0x000000000000000000000000000000000000bEEF", addr2)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}
