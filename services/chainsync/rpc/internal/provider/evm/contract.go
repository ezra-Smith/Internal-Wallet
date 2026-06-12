package evm

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

// CallContract calls a contract method with no state overrides.
func (p *Web3Provider) CallContract(ctx context.Context, contractAddress common.Address, data []byte) ([]byte, error) {
	start := time.Now()
	var err error
	defer p.finishRequest(start, &err)

	if p.client == nil {
		err = fmt.Errorf("ethereum client is nil")
		return nil, err
	}

	callMsg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	result, err := p.client.CallContract(ctx, callMsg, nil)
	if err != nil {
		err = fmt.Errorf("failed to call contract at %s: %v", contractAddress.Hex(), err)
		return nil, err
	}

	return result, nil
}
