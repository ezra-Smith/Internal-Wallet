package svc

import (
	"fmt"
	"strings"
	"time"

	"internalwallet/common/mq"
	"internalwallet/proto/pb"
)

type addressRegistryEventApplyResult struct {
	chain         pb.BlockChainType
	address       string
	source        mq.AddressMonitorSource
	action        mq.AddressMonitorAction
	before        addressSourceBits
	after         addressSourceBits
	monitoredAdd  bool
	monitoredDrop bool
}

func (ar *AddressRegistry) ApplyEvent(evt *mq.AddressMonitorEvent) (addressRegistryEventApplyResult, error) {
	if evt == nil {
		return addressRegistryEventApplyResult{}, fmt.Errorf("nil event")
	}

	// Respect current product requirements.
	if evt.Source == mq.AddressMonitorSourceWeb3 && !ar.shouldMonitorWeb3() {
		return addressRegistryEventApplyResult{
			chain:   pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED,
			address: "",
			source:  evt.Source,
			action:  evt.Action,
			before:  0,
			after:   0,
		}, nil
	}

	chainType := chainTypeFromString(evt.Chain)
	chainType = normalizeChainType(chainType)
	if chainType == pb.BlockChainType_CHAIN_TYPE_UNSPECIFIED {
		return addressRegistryEventApplyResult{}, fmt.Errorf("unsupported chain: %s", strings.TrimSpace(evt.Chain))
	}

	addr := normalizeMonitoredAddress(chainType, evt.Address)
	if addr == "" {
		return addressRegistryEventApplyResult{}, fmt.Errorf("empty address")
	}

	srcBit, ok := sourceBitsFromEventSource(evt.Source)
	if !ok {
		return addressRegistryEventApplyResult{}, fmt.Errorf("unsupported source: %s", string(evt.Source))
	}

	action := evt.Action
	if action == "" {
		return addressRegistryEventApplyResult{}, fmt.Errorf("empty action")
	}
	switch action {
	case mq.AddressMonitorActionUpsert, mq.AddressMonitorActionRemove:
	default:
		return addressRegistryEventApplyResult{}, fmt.Errorf("unsupported action: %s", string(action))
	}

	ar.mu.Lock()
	defer ar.mu.Unlock()

	snap := ar.Snapshot()
	prevDelta := ar.getDelta()

	before := ar.effectiveBits(chainType, addr, snap, prevDelta)

	nextDelta := cloneDeltaWithUpdate(prevDelta, chainType, addr, action, srcBit)
	after := ar.effectiveBits(chainType, addr, snap, nextDelta)

	ar.delta.Store(nextDelta)

	updatedSnap := *snap
	updatedSnap.lastEventAppliedAt = time.Now()
	updatedSnap.lastEventAppliedCount = 1
	if before == 0 && after != 0 {
		updatedSnap.lastEventAppliedAdded = 1
		updatedSnap.lastEventAppliedRemoved = 0
	} else if before != 0 && after == 0 {
		updatedSnap.lastEventAppliedAdded = 0
		updatedSnap.lastEventAppliedRemoved = 1
	} else {
		updatedSnap.lastEventAppliedAdded = 0
		updatedSnap.lastEventAppliedRemoved = 0
	}
	updatedSnap.lastEventAppliedSource = string(evt.Source)
	updatedSnap.lastEventAppliedChain = chainType
	updatedSnap.lastEventAppliedAddr = addr
	updatedSnap.lastEventAppliedAction = string(action)
	ar.snapshot.Store(&updatedSnap)

	return addressRegistryEventApplyResult{
		chain:         chainType,
		address:       addr,
		source:        evt.Source,
		action:        action,
		before:        before,
		after:         after,
		monitoredAdd:  before == 0 && after != 0,
		monitoredDrop: before != 0 && after == 0,
	}, nil
}

func sourceBitsFromEventSource(src mq.AddressMonitorSource) (addressSourceBits, bool) {
	switch src {
	case mq.AddressMonitorSourceDeposit:
		return addressSourceDeposit, true
	case mq.AddressMonitorSourceCompany:
		return addressSourceCompany, true
	case mq.AddressMonitorSourceVault:
		return addressSourceVault, true
	case mq.AddressMonitorSourceWeb3:
		return addressSourceWeb3, true
	case mq.AddressMonitorSourceManual:
		return addressSourceManual, true
	default:
		return 0, false
	}
}

func cloneDeltaWithUpdate(prev *addressRegistryDelta, chain pb.BlockChainType, addr string, action mq.AddressMonitorAction, srcBit addressSourceBits) *addressRegistryDelta {
	if prev == nil {
		prev = newEmptyAddressRegistryDelta()
	}

	next := &addressRegistryDelta{
		chains:      make(map[pb.BlockChainType]map[string]addressDelta, len(prev.chains)),
		lastApplied: time.Now(),
	}

	for ct, m := range prev.chains {
		next.chains[ct] = m
	}

	prevChain := prev.chains[chain]
	nextChain := make(map[string]addressDelta, len(prevChain)+1)
	for a, d := range prevChain {
		nextChain[a] = d
	}

	d := nextChain[addr]
	switch action {
	case mq.AddressMonitorActionUpsert:
		d.add |= srcBit
		d.remove &^= srcBit
	case mq.AddressMonitorActionRemove:
		d.remove |= srcBit
		d.add &^= srcBit
	default:
		return prev
	}

	if d.add == 0 && d.remove == 0 {
		delete(nextChain, addr)
	} else {
		nextChain[addr] = d
	}

	next.chains[chain] = nextChain
	return next
}
