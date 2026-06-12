package logic

import (
	"context"
	"fmt"
	"math/big"

	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTronAccountResourcesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger

	newTronClient func() (tronClientAPI, error)
}

func NewGetTronAccountResourcesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTronAccountResourcesLogic {
	return &GetTronAccountResourcesLogic{
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

// 查询 TRON 地址资源（Bandwidth/Energy/余额）
func (l *GetTronAccountResourcesLogic) GetTronAccountResources(in *pb.GetTronAccountResourcesReq) (*pb.GetTronAccountResourcesResp, error) {
	if in == nil {
		return &pb.GetTronAccountResourcesResp{
			Success: false,
			Message: "request is required",
		}, nil
	}

	addr, err := requireValidTronBase58Address("address", in.Address)
	if err != nil {
		return &pb.GetTronAccountResourcesResp{Success: false, Message: err.Error()}, nil
	}

	tron, err := l.newTronClient()
	if err != nil {
		return &pb.GetTronAccountResourcesResp{
			Success: false,
			Message: fmt.Sprintf("failed to get TRON client: %v", err),
		}, nil
	}
	defer tron.Stop()

	account, err := tron.GetAccount(addr)
	if err != nil {
		if isTronAccountNotFound(err) {
			// Unactivated address: return zeros (not an error).
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
		return &pb.GetTronAccountResourcesResp{
			Success: false,
			Message: fmt.Sprintf("failed to query account: %v", err),
		}, nil
	}

	res, err := tron.GetAccountResource(addr)
	if err != nil {
		return &pb.GetTronAccountResourcesResp{
			Success: false,
			Message: fmt.Sprintf("failed to query account resources: %v", err),
		}, nil
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
