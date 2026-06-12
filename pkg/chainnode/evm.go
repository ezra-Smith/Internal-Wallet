package chainnode

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"internalwallet/proto/pb"
)

var (
	erc20BalanceOfABI = `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}]`
	erc20TransferABI  = `[{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"}]`
	erc20SymbolABI    = `[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"}]`
	erc20DecimalsABI  = `[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`
)

func (c *directClient) GetBalance(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	if in == nil {
		return &pb.GetBalanceResp{Success: false, Message: "request is required"}, nil
	}
	if in.Chain != pb.ChainRpcType_CHAIN_TYPE_ETHEREUM && in.Chain != pb.ChainRpcType_CHAIN_TYPE_BSC && in.Chain != pb.ChainRpcType_CHAIN_TYPE_TRON {
		return &pb.GetBalanceResp{Success: false, Message: "unsupported chain"}, nil
	}
	if in.Chain == pb.ChainRpcType_CHAIN_TYPE_TRON {
		return c.getTronBalance(ctx, in)
	}
	return c.getEvmBalance(ctx, in)
}

func (c *directClient) getEvmBalance(ctx context.Context, in *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	if strings.TrimSpace(in.Address) == "" {
		return &pb.GetBalanceResp{Success: false, Message: "address is required"}, nil
	}
	if !common.IsHexAddress(in.Address) {
		return &pb.GetBalanceResp{Success: false, Message: "invalid address"}, nil
	}

	cli, err := c.getEVM(in.Chain)
	if err != nil {
		return &pb.GetBalanceResp{Success: false, Message: err.Error()}, nil
	}

	addr := common.HexToAddress(in.Address)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var (
		bal *big.Int
		bn  uint64
	)
	if in.BlockNumber > 0 {
		bal, err = cli.BalanceAt(reqCtx, addr, big.NewInt(int64(in.BlockNumber)))
		bn = in.BlockNumber
	} else {
		bal, err = cli.BalanceAt(reqCtx, addr, nil)
		if err == nil {
			if h, hErr := cli.BlockNumber(reqCtx); hErr == nil {
				bn = h
			}
		}
	}
	if err != nil {
		return &pb.GetBalanceResp{Success: false, Message: fmt.Sprintf("BalanceAt failed: %v", err)}, nil
	}

	return &pb.GetBalanceResp{
		Success:     true,
		Message:     "ok",
		Balance:     bal.String(),
		BlockNumber: bn,
	}, nil
}

func (c *directClient) GetTokenBalance(ctx context.Context, in *pb.GetTokenBalanceReq) (*pb.GetTokenBalanceResp, error) {
	if in == nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: "request is required"}, nil
	}
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return c.getEvmTokenBalance(ctx, in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return c.getTronTokenBalance(ctx, in)
	default:
		return &pb.GetTokenBalanceResp{Success: false, Message: "unsupported chain"}, nil
	}
}

func (c *directClient) getEvmTokenBalance(ctx context.Context, in *pb.GetTokenBalanceReq) (*pb.GetTokenBalanceResp, error) {
	if strings.TrimSpace(in.Address) == "" || strings.TrimSpace(in.TokenContract) == "" {
		return &pb.GetTokenBalanceResp{Success: false, Message: "address and token_contract are required"}, nil
	}
	if !common.IsHexAddress(in.Address) || !common.IsHexAddress(in.TokenContract) {
		return &pb.GetTokenBalanceResp{Success: false, Message: "invalid address/token_contract"}, nil
	}

	cli, err := c.getEVM(in.Chain)
	if err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: err.Error()}, nil
	}

	parsedABI, err := abi.JSON(strings.NewReader(erc20BalanceOfABI))
	if err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: fmt.Sprintf("parse abi: %v", err)}, nil
	}

	owner := common.HexToAddress(in.Address)
	contract := common.HexToAddress(in.TokenContract)

	data, err := parsedABI.Pack("balanceOf", owner)
	if err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: fmt.Sprintf("pack balanceOf: %v", err)}, nil
	}

	var blockNumber *big.Int
	if strings.TrimSpace(in.BlockNumber) != "" {
		if bn, ok := new(big.Int).SetString(strings.TrimSpace(in.BlockNumber), 10); ok {
			blockNumber = bn
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	result, err := cli.CallContract(reqCtx, ethereum.CallMsg{To: &contract, Data: data}, blockNumber)
	if err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: fmt.Sprintf("call balanceOf: %v", err)}, nil
	}

	var balance *big.Int
	if err := parsedABI.UnpackIntoInterface(&balance, "balanceOf", result); err != nil {
		return &pb.GetTokenBalanceResp{Success: false, Message: fmt.Sprintf("unpack balanceOf: %v", err)}, nil
	}

	return &pb.GetTokenBalanceResp{
		Success: true,
		Message: "ok",
		Balance: balance.String(),
	}, nil
}

func (c *directClient) EstimateGas(ctx context.Context, in *pb.EstimateGasReq) (*pb.EstimateGasResp, error) {
	if in == nil {
		return &pb.EstimateGasResp{Success: false, Message: "request is required"}, nil
	}
	if in.Chain != pb.ChainRpcType_CHAIN_TYPE_ETHEREUM && in.Chain != pb.ChainRpcType_CHAIN_TYPE_BSC {
		return &pb.EstimateGasResp{Success: false, Message: "only ETH/BSC supported for EstimateGas"}, nil
	}

	if strings.TrimSpace(in.FromAddress) == "" {
		return &pb.EstimateGasResp{Success: false, Message: "from_address is required"}, nil
	}
	if !common.IsHexAddress(in.FromAddress) {
		return &pb.EstimateGasResp{Success: false, Message: "invalid from_address"}, nil
	}

	cli, err := c.getEVM(in.Chain)
	if err != nil {
		return &pb.EstimateGasResp{Success: false, Message: err.Error()}, nil
	}

	from := common.HexToAddress(in.FromAddress)
	var (
		to    *common.Address
		value = big.NewInt(0)
		data  []byte
	)

	// Parse value (decimal string, smallest unit).
	if strings.TrimSpace(in.Value) != "" {
		v, ok := new(big.Int).SetString(strings.TrimSpace(in.Value), 10)
		if !ok {
			return &pb.EstimateGasResp{Success: false, Message: "invalid value"}, nil
		}
		value = v
	}

	// Build callmsg.
	if strings.TrimSpace(in.TokenContract) != "" {
		// ERC20 transfer(to, amount)
		if strings.TrimSpace(in.ToAddress) == "" {
			return &pb.EstimateGasResp{Success: false, Message: "to_address is required for token transfer"}, nil
		}
		if !common.IsHexAddress(in.ToAddress) || !common.IsHexAddress(in.TokenContract) {
			return &pb.EstimateGasResp{Success: false, Message: "invalid to_address/token_contract"}, nil
		}

		parsedABI, err := abi.JSON(strings.NewReader(erc20TransferABI))
		if err != nil {
			return &pb.EstimateGasResp{Success: false, Message: fmt.Sprintf("parse abi: %v", err)}, nil
		}
		toRecipient := common.HexToAddress(in.ToAddress)
		data, err = parsedABI.Pack("transfer", toRecipient, value)
		if err != nil {
			return &pb.EstimateGasResp{Success: false, Message: fmt.Sprintf("pack transfer: %v", err)}, nil
		}
		tokenContract := common.HexToAddress(in.TokenContract)
		to = &tokenContract
		value = big.NewInt(0)
	} else {
		if strings.TrimSpace(in.ToAddress) != "" {
			if !common.IsHexAddress(in.ToAddress) {
				return &pb.EstimateGasResp{Success: false, Message: "invalid to_address"}, nil
			}
			addr := common.HexToAddress(in.ToAddress)
			to = &addr
		}
		if strings.TrimSpace(in.Data) != "" {
			b, err := hexutil.Decode(strings.TrimSpace(in.Data))
			if err != nil {
				return &pb.EstimateGasResp{Success: false, Message: fmt.Sprintf("invalid data: %v", err)}, nil
			}
			data = b
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	gasPrice, err := cli.SuggestGasPrice(reqCtx)
	if err != nil {
		return &pb.EstimateGasResp{Success: false, Message: fmt.Sprintf("SuggestGasPrice failed: %v", err)}, nil
	}

	msg := ethereum.CallMsg{From: from, To: to, Value: value, Data: data}
	gasLimit, err := cli.EstimateGas(reqCtx, msg)
	if err != nil {
		return &pb.EstimateGasResp{Success: false, Message: fmt.Sprintf("EstimateGas failed: %v", err)}, nil
	}

	estimatedFee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	return &pb.EstimateGasResp{
		Success:      true,
		Message:      "ok",
		GasLimit:     gasLimit,
		GasPrice:     gasPrice.String(),
		EstimatedFee: estimatedFee.String(),
	}, nil
}

func (c *directClient) SuggestGasPrice(ctx context.Context, chain pb.ChainRpcType) (string, error) {
	if chain != pb.ChainRpcType_CHAIN_TYPE_ETHEREUM && chain != pb.ChainRpcType_CHAIN_TYPE_BSC {
		return "", fmt.Errorf("only ETH/BSC supported for SuggestGasPrice")
	}
	cli, err := c.getEVM(chain)
	if err != nil {
		return "", err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	gasPrice, err := cli.SuggestGasPrice(reqCtx)
	if err != nil {
		return "", fmt.Errorf("SuggestGasPrice failed: %w", err)
	}
	return gasPrice.String(), nil
}

func (c *directClient) BuildTransaction(ctx context.Context, in *pb.BuildTransactionReq) (*pb.BuildTransactionResp, error) {
	if in == nil {
		return &pb.BuildTransactionResp{Success: false, Message: "request is required"}, nil
	}
	if in.Chain != pb.ChainRpcType_CHAIN_TYPE_ETHEREUM && in.Chain != pb.ChainRpcType_CHAIN_TYPE_BSC {
		return &pb.BuildTransactionResp{Success: false, Message: "only ETH/BSC are supported for BuildTransaction"}, nil
	}
	if strings.TrimSpace(in.FromAddress) == "" || strings.TrimSpace(in.ToAddress) == "" {
		return &pb.BuildTransactionResp{Success: false, Message: "from_address and to_address are required"}, nil
	}
	if !common.IsHexAddress(in.FromAddress) || !common.IsHexAddress(in.ToAddress) {
		return &pb.BuildTransactionResp{Success: false, Message: "invalid from_address/to_address"}, nil
	}
	if strings.TrimSpace(in.Amount) == "" {
		return &pb.BuildTransactionResp{Success: false, Message: "amount is required"}, nil
	}

	cli, err := c.getEVM(in.Chain)
	if err != nil {
		return &pb.BuildTransactionResp{Success: false, Message: err.Error()}, nil
	}

	from := common.HexToAddress(in.FromAddress)
	toRecipient := common.HexToAddress(in.ToAddress)

	amount, ok := new(big.Int).SetString(strings.TrimSpace(in.Amount), 10)
	if !ok {
		return &pb.BuildTransactionResp{Success: false, Message: "invalid amount"}, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Nonce
	var nonce uint64
	if strings.TrimSpace(in.Nonce) != "" {
		n, err := strconv.ParseUint(strings.TrimSpace(in.Nonce), 10, 64)
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("invalid nonce: %v", err)}, nil
		}
		nonce = n
	} else {
		n, err := cli.PendingNonceAt(reqCtx, from)
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("PendingNonceAt failed: %v", err)}, nil
		}
		nonce = n
	}

	// Gas price
	var gasPrice *big.Int
	if strings.TrimSpace(in.GasPrice) != "" {
		gp, ok := new(big.Int).SetString(strings.TrimSpace(in.GasPrice), 10)
		if !ok {
			return &pb.BuildTransactionResp{Success: false, Message: "invalid gas_price"}, nil
		}
		gasPrice = gp
	} else {
		gp, err := cli.SuggestGasPrice(reqCtx)
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("SuggestGasPrice failed: %v", err)}, nil
		}
		gasPrice = gp
	}

	var (
		to    common.Address
		value *big.Int
		data  []byte
	)
	if strings.TrimSpace(in.TokenContract) != "" {
		if !common.IsHexAddress(in.TokenContract) {
			return &pb.BuildTransactionResp{Success: false, Message: "invalid token_contract"}, nil
		}
		parsedABI, err := abi.JSON(strings.NewReader(erc20TransferABI))
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("parse ERC20 ABI: %v", err)}, nil
		}
		data, err = parsedABI.Pack("transfer", toRecipient, amount)
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("pack transfer: %v", err)}, nil
		}
		to = common.HexToAddress(in.TokenContract)
		value = big.NewInt(0)
	} else {
		to = toRecipient
		value = amount
	}

	gasLimit := in.GasLimit
	if gasLimit == 0 {
		msg := ethereum.CallMsg{From: from, To: &to, Value: value, Data: data}
		estimated, err := cli.EstimateGas(reqCtx, msg)
		if err != nil {
			if strings.TrimSpace(in.TokenContract) != "" {
				gasLimit = 60_000
			} else {
				gasLimit = 21_000
			}
		} else {
			gasLimit = estimated
		}
	}

	// Legacy tx (type 0).
	tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)
	rawTxBytes, err := tx.MarshalBinary()
	if err != nil {
		return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("marshal tx: %v", err)}, nil
	}

	estimatedFee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	return &pb.BuildTransactionResp{
		Success:        true,
		Message:        "ok",
		RawTransaction: hexutil.Encode(rawTxBytes),
		Nonce:          nonce,
		GasLimit:       gasLimit,
		GasPrice:       gasPrice.String(),
		EstimatedFee:   estimatedFee.String(),
	}, nil
}

func (c *directClient) GetBlockHeight(ctx context.Context, in *pb.GetBlockHeightReq) (*pb.GetBlockHeightResp, error) {
	if in == nil {
		return &pb.GetBlockHeightResp{Success: false, Message: "request is required"}, nil
	}
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		cli, err := c.getEVM(in.Chain)
		if err != nil {
			return &pb.GetBlockHeightResp{Success: false, Message: err.Error()}, nil
		}
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		h, err := cli.BlockNumber(reqCtx)
		if err != nil {
			return &pb.GetBlockHeightResp{Success: false, Message: fmt.Sprintf("BlockNumber failed: %v", err)}, nil
		}
		return &pb.GetBlockHeightResp{
			Success:     true,
			Message:     "ok",
			BlockHeight: h,
			Timestamp:   time.Now().Unix(),
		}, nil
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return c.getTronBlockHeight(ctx)
	default:
		return &pb.GetBlockHeightResp{Success: false, Message: "unsupported chain"}, nil
	}
}

func (c *directClient) GetTransactionReceipt(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
	if in == nil {
		return &pb.GetTransactionReceiptResp{Success: false, Message: "request is required"}, nil
	}
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return c.getEvmTransactionReceipt(ctx, in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return c.getTronTransactionReceipt(ctx, in)
	default:
		return &pb.GetTransactionReceiptResp{Success: false, Message: "unsupported chain"}, nil
	}
}

func (c *directClient) getEvmTransactionReceipt(ctx context.Context, in *pb.GetTransactionReceiptReq) (*pb.GetTransactionReceiptResp, error) {
	txHashStr := strings.TrimSpace(in.TxHash)
	if txHashStr == "" {
		return &pb.GetTransactionReceiptResp{Success: false, Message: "tx_hash is required"}, nil
	}
	cli, err := c.getEVM(in.Chain)
	if err != nil {
		return &pb.GetTransactionReceiptResp{Success: false, Message: err.Error()}, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	rcpt, err := cli.TransactionReceipt(reqCtx, common.HexToHash(txHashStr))
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return &pb.GetTransactionReceiptResp{Success: true, Message: "receipt not found"}, nil
		}
		return &pb.GetTransactionReceiptResp{Success: false, Message: fmt.Sprintf("TransactionReceipt failed: %v", err)}, nil
	}
	if rcpt == nil {
		return &pb.GetTransactionReceiptResp{Success: true, Message: "receipt not found"}, nil
	}

	status := pb.TxStatus_TX_STATUS_FAILED
	if rcpt.Status == 1 {
		status = pb.TxStatus_TX_STATUS_CONFIRMED
	}

	var gasFee string
	if rcpt.EffectiveGasPrice != nil && rcpt.GasUsed > 0 {
		gasFee = new(big.Int).Mul(new(big.Int).SetUint64(rcpt.GasUsed), rcpt.EffectiveGasPrice).String()
	}

	return &pb.GetTransactionReceiptResp{
		Success: true,
		Message: "ok",
		Receipt: &pb.TransactionReceipt{
			TxHash:           txHashStr,
			BlockNumber:      rcpt.BlockNumber.String(),
			BlockHash:        rcpt.BlockHash.Hex(),
			TransactionIndex: uint64(rcpt.TransactionIndex),
			GasUsed:          rcpt.GasUsed,
			GasFee:           gasFee,
			Status:           status,
			ContractAddress:  rcpt.ContractAddress.Hex(),
		},
	}, nil
}

func (c *directClient) GetTokenInfo(ctx context.Context, in *pb.GetTokenInfoReq) (*pb.GetTokenInfoResp, error) {
	if in == nil {
		return &pb.GetTokenInfoResp{Success: false, Message: "request is required"}, nil
	}
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return c.getEvmTokenInfo(ctx, in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return c.getTronTokenInfo(ctx, in)
	default:
		return &pb.GetTokenInfoResp{Success: false, Message: "unsupported chain"}, nil
	}
}

func (c *directClient) getEvmTokenInfo(ctx context.Context, in *pb.GetTokenInfoReq) (*pb.GetTokenInfoResp, error) {
	contractStr := strings.TrimSpace(in.TokenContract)
	if contractStr == "" {
		return &pb.GetTokenInfoResp{Success: false, Message: "token_contract is required"}, nil
	}
	if !common.IsHexAddress(contractStr) {
		return &pb.GetTokenInfoResp{Success: false, Message: "invalid token_contract"}, nil
	}

	cli, err := c.getEVM(in.Chain)
	if err != nil {
		return &pb.GetTokenInfoResp{Success: false, Message: err.Error()}, nil
	}

	contract := common.HexToAddress(contractStr)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	symbol, _ := evmCallString(reqCtx, cli, contract, erc20SymbolABI, "symbol")
	decimals, _ := evmCallUint8(reqCtx, cli, contract, erc20DecimalsABI, "decimals")

	ti := &pb.TokenInfo{
		ContractAddress: contractStr,
		Symbol:          symbol,
		Decimals:        uint32(decimals),
	}
	return &pb.GetTokenInfoResp{Success: true, Message: "ok", TokenInfo: ti}, nil
}

func evmCallString(ctx context.Context, cli *ethclient.Client, contract common.Address, abiJSON string, fn string) (string, error) {
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return "", err
	}
	data, err := parsed.Pack(fn)
	if err != nil {
		return "", err
	}
	out, err := cli.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: data}, nil)
	if err != nil {
		return "", err
	}
	var s string
	if err := parsed.UnpackIntoInterface(&s, fn, out); err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func evmCallUint8(ctx context.Context, cli *ethclient.Client, contract common.Address, abiJSON string, fn string) (uint8, error) {
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return 0, err
	}
	data, err := parsed.Pack(fn)
	if err != nil {
		return 0, err
	}
	out, err := cli.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: data}, nil)
	if err != nil {
		return 0, err
	}
	var v uint8
	if err := parsed.UnpackIntoInterface(&v, fn, out); err != nil {
		return 0, err
	}
	return v, nil
}

func (c *directClient) BroadcastTransaction(ctx context.Context, in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error) {
	if in == nil {
		return &pb.BroadcastTransactionResp{Success: false, Message: "request is required"}, nil
	}
	if strings.TrimSpace(in.SignedTransaction) == "" {
		return &pb.BroadcastTransactionResp{Success: false, Message: "signed_transaction is required"}, nil
	}
	switch in.Chain {
	case pb.ChainRpcType_CHAIN_TYPE_ETHEREUM, pb.ChainRpcType_CHAIN_TYPE_BSC:
		return c.broadcastEvm(ctx, in)
	case pb.ChainRpcType_CHAIN_TYPE_TRON:
		return c.broadcastTron(ctx, in)
	default:
		return &pb.BroadcastTransactionResp{Success: false, Message: "unsupported chain"}, nil
	}
}

func (c *directClient) broadcastEvm(ctx context.Context, in *pb.BroadcastTransactionReq) (*pb.BroadcastTransactionResp, error) {
	cli, err := c.getEVM(in.Chain)
	if err != nil {
		return &pb.BroadcastTransactionResp{Success: false, Message: err.Error()}, nil
	}

	signedTxBytes, err := hexutil.Decode(strings.TrimSpace(in.SignedTransaction))
	if err != nil {
		return &pb.BroadcastTransactionResp{Success: false, Message: fmt.Sprintf("decode signed tx: %v", err)}, nil
	}
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(signedTxBytes); err != nil {
		return &pb.BroadcastTransactionResp{Success: false, Message: fmt.Sprintf("unmarshal tx: %v", err)}, nil
	}
	txHash := tx.Hash().Hex()

	timeoutSeconds := in.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	if err := cli.SendTransaction(reqCtx, tx); err != nil {
		return &pb.BroadcastTransactionResp{
			Success:       false,
			Message:       fmt.Sprintf("broadcast failed: %v", err),
			TxHash:        txHash,
			Nonce:         tx.Nonce(),
			Status:        pb.TxStatus_TX_STATUS_FAILED,
			BroadcastedAt: time.Now().Unix(),
		}, nil
	}

	return &pb.BroadcastTransactionResp{
		Success:       true,
		Message:       "ok",
		TxHash:        txHash,
		Nonce:         tx.Nonce(),
		Status:        pb.TxStatus_TX_STATUS_PENDING,
		BroadcastedAt: time.Now().Unix(),
	}, nil
}

func parseUint64Str(s string) uint64 {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return v
}
