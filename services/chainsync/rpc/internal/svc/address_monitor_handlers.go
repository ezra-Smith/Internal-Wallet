package svc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	chainModel "internalwallet/services/chainsync/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

// AddMonitor 添加地址监控（实现接口）
func (am *AddressMonitor) AddMonitor(req *pb.AddAddressMonitorReq) (*pb.AddAddressMonitorResp, error) {
	if req == nil {
		return &pb.AddAddressMonitorResp{
			Success:   false,
			Message:   "invalid request",
			MonitorId: "",
		}, fmt.Errorf("nil request")
	}
	if req.Address == "" {
		return &pb.AddAddressMonitorResp{
			Success:   false,
			Message:   "address cannot be empty",
			MonitorId: "",
		}, fmt.Errorf("address cannot be empty")
	}
	if req.Chain == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
		return &pb.AddAddressMonitorResp{
			Success:   false,
			Message:   "chain is required",
			MonitorId: "",
		}, fmt.Errorf("chain cannot be unspecified")
	}

	chainType := req.Chain
	address := normalizeMonitoredAddress(chainType, req.Address)
	if address == "" {
		return &pb.AddAddressMonitorResp{
			Success:   false,
			Message:   "address cannot be empty",
			MonitorId: "",
		}, fmt.Errorf("address cannot be empty")
	}

	chainStr := chainStringForDB(chainType)
	if chainStr == "" {
		return &pb.AddAddressMonitorResp{
			Success:   false,
			Message:   "unsupported chain",
			MonitorId: "",
		}, fmt.Errorf("unsupported chain: %v", chainType)
	}

	// Manual/supplementary monitors are persisted in chainsync DB when available.
	monitorID := fmt.Sprintf("manual_%s_%s", chainStr, address)
	if am.addressMonitorRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if existing, err := am.addressMonitorRepo.GetByMonitorID(ctx, monitorID); err == nil && existing != nil {
			existing.Active = true
			existing.Chain = chainStr
			existing.Address = address
			existing.MonitorType = "all"
			existing.Priority = "normal"
			existing.Tag = strings.TrimSpace(req.Tag)
			existing.WebhookURL = strings.TrimSpace(req.WebhookUrl)
			if req.Metadata != nil {
				existing.Metadata = chainModel.Metadata(req.Metadata)
			}
			if err := am.addressMonitorRepo.Update(ctx, existing); err != nil {
				logx.Errorf("Failed to update manual monitor (monitor_id=%s): %v", monitorID, err)
			}
		} else {
			m := &chainModel.AddressMonitorGorm{
				MonitorID:   monitorID,
				Chain:       chainStr,
				Address:     address,
				MonitorType: "all",
				Priority:    "normal",
				Tag:         strings.TrimSpace(req.Tag),
				WebhookURL:  strings.TrimSpace(req.WebhookUrl),
				Active:      true,
			}
			if req.Metadata != nil {
				m.Metadata = chainModel.Metadata(req.Metadata)
			}
			if err := am.addressMonitorRepo.Create(ctx, m); err != nil {
				logx.Errorf("Failed to persist manual monitor (monitor_id=%s): %v", monitorID, err)
			}
		}
	}

	logx.Infof("✅ Added manual monitor for address: %s on chain %v (monitor_id=%s)", address, chainType, monitorID)

	return &pb.AddAddressMonitorResp{
		Success:   true,
		Message:   "ok",
		MonitorId: monitorID,
	}, nil
}

// RemoveMonitor 移除地址监控（实现接口）
func (am *AddressMonitor) RemoveMonitor(req *pb.RemoveAddressMonitorReq) (*pb.RemoveAddressMonitorResp, error) {
	if req == nil {
		return &pb.RemoveAddressMonitorResp{
			Success: false,
			Message: "invalid request",
		}, fmt.Errorf("nil request")
	}
	chainType := req.Chain
	if chainType == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
		return &pb.RemoveAddressMonitorResp{
			Success: false,
			Message: "chain is required",
		}, fmt.Errorf("chain cannot be unspecified")
	}

	address := normalizeMonitoredAddress(chainType, req.Address)
	if address == "" {
		return &pb.RemoveAddressMonitorResp{
			Success: false,
			Message: "address is required",
		}, fmt.Errorf("address cannot be empty")
	}
	chainStr := chainStringForDB(chainType)
	monitorID := fmt.Sprintf("manual_%s_%s", chainStr, address)
	if am.addressMonitorRepo != nil && chainStr != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if existing, err := am.addressMonitorRepo.GetByMonitorID(ctx, monitorID); err == nil && existing != nil {
			existing.Active = false
			if err := am.addressMonitorRepo.Update(ctx, existing); err != nil {
				logx.Errorf("Failed to deactivate manual monitor (monitor_id=%s): %v", monitorID, err)
			}
		}
	}

	logx.Infof("✅ Removed manual monitor for address: %s on chain %v (monitor_id=%s)", address, chainType, monitorID)

	return &pb.RemoveAddressMonitorResp{
		Success: true,
		Message: "ok",
	}, nil
}

// GetMonitors 获取监控地址列表（实现接口）
func (am *AddressMonitor) GetMonitors(req *pb.GetMonitoredAddressesReq) (*pb.GetMonitoredAddressesResp, error) {
	if am.registry == nil {
		return &pb.GetMonitoredAddressesResp{Success: true}, nil
	}

	page := int32(1)
	pageSize := int32(100)
	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.PageSize > 0 {
			pageSize = req.PageSize
		}
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	offset := int((page - 1) * pageSize)
	limit := int(pageSize)

	snap := am.registry.Snapshot()

	var chains []pb.BlockChainType
	if req != nil && req.Chain != pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
		chains = []pb.BlockChainType{req.Chain}
	} else {
		chains = []pb.BlockChainType{
			pb.BlockChainType_CHAIN_TYPE_ETHEREUM,
			pb.BlockChainType_CHAIN_TYPE_BSC,
			pb.BlockChainType_CHAIN_TYPE_TRON,
		}
	}

	total := 0
	activeCount := 0
	monitors := make([]*pb.AddressMonitor, 0, limit)
	now := time.Now().Unix()

	for _, chainType := range chains {
		cs := snap.chains[chainType]
		if cs == nil || len(cs.addresses) == 0 {
			continue
		}
		for _, addr := range cs.addresses {
			total++
			activeCount++
			if total <= offset {
				continue
			}
			if len(monitors) >= limit {
				continue
			}
			monitors = append(monitors, &pb.AddressMonitor{
				MonitorId:    fmt.Sprintf("monitor_%s_%s", chainType, addr),
				Address:      addr,
				Chain:        chainType,
				MonitorType:  pb.MonitorType_MONITOR_TYPE_ALL,
				Priority:     pb.Priority_PRIORITY_NORMAL,
				Active:       true,
				Tag:          "monitored",
				CreatedAt:    now,
				LastActivity: 0,
			})
		}
	}

	return &pb.GetMonitoredAddressesResp{
		Success:     true,
		Monitors:    monitors,
		Total:       int32(total),
		ActiveCount: int32(activeCount),
	}, nil
}

func (am *AddressMonitor) GetAddressMonitorStats(_ *pb.GetAddressMonitorStatsReq) (*pb.GetAddressMonitorStatsResp, error) {
	if am == nil || am.registry == nil {
		return &pb.GetAddressMonitorStatsResp{
			Success: true,
			Message: "ok",
			Data:    &pb.AddressMonitorStatsData{},
		}, nil
	}

	snap := am.registry.Snapshot()
	delta := am.registry.getDelta()
	treatWeb3AsInternal := am.registry.shouldTreatWeb3AsInternal()

	unixSeconds := func(t time.Time) int64 {
		if t.IsZero() {
			return 0
		}
		return t.Unix()
	}

	data := &pb.AddressMonitorStatsData{
		LastFullRefreshAt:         unixSeconds(snap.lastFullRefreshAt),
		LastFullRefreshDurationMs: int64(snap.lastFullRefreshDuration / time.Millisecond),
		LastFullRefreshAdded:      int32(snap.lastFullRefreshAdded),
		LastFullRefreshRemoved:    int32(snap.lastFullRefreshRemoved),
		LastEventAppliedAt:        unixSeconds(snap.lastEventAppliedAt),
		LastEventAppliedAdded:     int32(snap.lastEventAppliedAdded),
		LastEventAppliedRemoved:   int32(snap.lastEventAppliedRemoved),
		LastEventSource:           snap.lastEventAppliedSource,
		LastEventChain:            snap.lastEventAppliedChain,
		LastEventAddress:          snap.lastEventAppliedAddr,
		LastEventAction:           snap.lastEventAppliedAction,
		Chains:                    nil,
	}

	chainList := []pb.BlockChainType{
		pb.BlockChainType_CHAIN_TYPE_ETHEREUM,
		pb.BlockChainType_CHAIN_TYPE_BSC,
		pb.BlockChainType_CHAIN_TYPE_TRON,
	}

	overallMonitored := int32(0)
	overallInternal := int32(0)
	overallDeposit := int32(0)
	overallCompany := int32(0)
	overallVault := int32(0)
	overallWeb3 := int32(0)
	overallManual := int32(0)

	for _, chainType := range chainList {
		cs := snap.chains[chainType]
		var baseAddrs []string
		var baseMap map[string]addressSourceBits
		if cs != nil {
			baseAddrs = cs.addresses
			baseMap = cs.sourceBitsByAddr
		}
		if baseMap == nil {
			baseMap = map[string]addressSourceBits{}
		}

		monitored := int32(0)
		internal := int32(0)
		deposit := int32(0)
		company := int32(0)
		vault := int32(0)
		web3 := int32(0)
		manual := int32(0)

		countAddress := func(addr string) {
			bits := am.registry.effectiveBits(chainType, addr, snap, delta)
			if bits == 0 {
				return
			}
			monitored++

			if bits&addressSourceDeposit != 0 {
				deposit++
			}
			if bits&addressSourceCompany != 0 {
				company++
			}
			if bits&addressSourceVault != 0 {
				vault++
			}
			if bits&addressSourceWeb3 != 0 {
				web3++
			}
			if bits&addressSourceManual != 0 {
				manual++
			}

			isInternal := bits&(addressSourceDeposit|addressSourceCompany|addressSourceVault) != 0
			if !isInternal && treatWeb3AsInternal && bits&addressSourceWeb3 != 0 {
				isInternal = true
			}
			if isInternal {
				internal++
			}
		}

		for _, addr := range baseAddrs {
			countAddress(addr)
		}

		deltaEntries := int32(0)
		if deltaChain := delta.chains[chainType]; deltaChain != nil {
			deltaEntries = int32(len(deltaChain))
			for addr := range deltaChain {
				if _, inBase := baseMap[addr]; inBase {
					continue
				}
				countAddress(addr)
			}
		}

		data.Chains = append(data.Chains, &pb.AddressMonitorChainStats{
			Chain:          chainType,
			MonitoredTotal: monitored,
			InternalTotal:  internal,
			DepositTotal:   deposit,
			CompanyTotal:   company,
			VaultTotal:     vault,
			Web3Total:      web3,
			ManualTotal:    manual,
			DeltaEntries:   deltaEntries,
		})

		overallMonitored += monitored
		overallInternal += internal
		overallDeposit += deposit
		overallCompany += company
		overallVault += vault
		overallWeb3 += web3
		overallManual += manual
	}

	data.MonitoredTotal = overallMonitored
	data.InternalTotal = overallInternal
	data.DepositTotal = overallDeposit
	data.CompanyTotal = overallCompany
	data.VaultTotal = overallVault
	data.Web3Total = overallWeb3
	data.ManualTotal = overallManual

	return &pb.GetAddressMonitorStatsResp{
		Success: true,
		Message: "ok",
		Data:    data,
	}, nil
}
