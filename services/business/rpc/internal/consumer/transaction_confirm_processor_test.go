package consumer

import (
	"context"
	"testing"

	"internalwallet/common/mq"

	"github.com/stretchr/testify/require"
)

func TestTransactionConfirmProcessor_Process_DepositSkipInternalCounterparty(t *testing.T) {
	p := NewTransactionConfirmProcessor(nil)
	msg := &mq.TransactionConfirmMessage{
		Source:                 mq.AddressMonitorSourceDeposit,
		Direction:              "in",
		CounterpartyIsInternal: true,
	}
	require.NoError(t, p.Process(context.Background(), msg))
}

func TestTransactionConfirmProcessor_Process_DepositSkipNonInbound(t *testing.T) {
	p := NewTransactionConfirmProcessor(nil)
	msg := &mq.TransactionConfirmMessage{
		Source:    mq.AddressMonitorSourceDeposit,
		Direction: "out",
	}
	require.NoError(t, p.Process(context.Background(), msg))
}
