package logic

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"internalwallet/proto/pb"
	"internalwallet/services/chainrpc/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type BuildTransactionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBuildTransactionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BuildTransactionLogic {
	return &BuildTransactionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

var buildErc20TransferABI = `[{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"payable":false,"stateMutability":"nonpayable","type":"function"}]`

// 构建 EVM 交易（ETH/BSC，返回未签名 rawTx，供 Signer 签名）
func (l *BuildTransactionLogic) BuildTransaction(in *pb.BuildTransactionReq) (*pb.BuildTransactionResp, error) {
	if in == nil {
		return &pb.BuildTransactionResp{Success: false, Message: "request is required"}, nil
	}
	if in.Chain != pb.ChainRpcType_CHAIN_TYPE_ETHEREUM && in.Chain != pb.ChainRpcType_CHAIN_TYPE_BSC {
		return &pb.BuildTransactionResp{Success: false, Message: "only ETH/BSC are supported for BuildTransaction"}, nil
	}
	if strings.TrimSpace(in.FromAddress) == "" || strings.TrimSpace(in.ToAddress) == "" {
		return &pb.BuildTransactionResp{Success: false, Message: "from_address and to_address are required"}, nil
	}
	if !common.IsHexAddress(in.FromAddress) {
		return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("invalid from_address: %s", in.FromAddress)}, nil
	}
	if !common.IsHexAddress(in.ToAddress) {
		return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("invalid to_address: %s", in.ToAddress)}, nil
	}
	if strings.TrimSpace(in.Amount) == "" {
		return &pb.BuildTransactionResp{Success: false, Message: "amount is required"}, nil
	}

	ethClient, err := l.svcCtx.ChainMgr.GetETHClient(in.Chain)
	if err != nil {
		return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("failed to get ETH client: %v", err)}, nil
	}

	fromAddr := common.HexToAddress(in.FromAddress)
	toRecipient := common.HexToAddress(in.ToAddress)

	amount, ok := new(big.Int).SetString(in.Amount, 10)
	if !ok {
		return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("invalid amount: %s", in.Amount)}, nil
	}

	// nonce
	var nonce uint64
	if strings.TrimSpace(in.Nonce) != "" {
		n, err := strconv.ParseUint(strings.TrimSpace(in.Nonce), 10, 64)
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("invalid nonce: %v", err)}, nil
		}
		nonce = n
	} else {
		n, err := ethClient.PendingNonceAt(context.Background(), fromAddr)
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("failed to get nonce: %v", err)}, nil
		}
		nonce = n
	}

	// gas price
	var gasPrice *big.Int
	if strings.TrimSpace(in.GasPrice) != "" {
		gp, ok := new(big.Int).SetString(strings.TrimSpace(in.GasPrice), 10)
		if !ok {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("invalid gas_price: %s", in.GasPrice)}, nil
		}
		gasPrice = gp
	} else {
		gp, err := ethClient.SuggestGasPrice(context.Background())
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("failed to get gas price: %v", err)}, nil
		}
		gasPrice = gp
	}

	// build tx fields
	var (
		to    common.Address
		value *big.Int
		data  []byte
	)
	if strings.TrimSpace(in.TokenContract) != "" {
		if !common.IsHexAddress(in.TokenContract) {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("invalid token_contract: %s", in.TokenContract)}, nil
		}
		tokenContract := common.HexToAddress(in.TokenContract)
		parsedABI, err := abi.JSON(strings.NewReader(buildErc20TransferABI))
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("failed to parse ERC20 ABI: %v", err)}, nil
		}
		data, err = parsedABI.Pack("transfer", toRecipient, amount)
		if err != nil {
			return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("failed to pack ERC20 transfer: %v", err)}, nil
		}
		to = tokenContract
		value = big.NewInt(0)
	} else {
		to = toRecipient
		value = amount
	}

	// gas limit
	gasLimit := in.GasLimit
	if gasLimit == 0 {
		msg := ethereum.CallMsg{
			From:  fromAddr,
			To:    &to,
			Value: value,
			Data:  data,
		}
		estimated, err := ethClient.EstimateGas(context.Background(), msg)
		if err != nil {
			// fallback defaults
			if strings.TrimSpace(in.TokenContract) != "" {
				gasLimit = 60000
			} else {
				gasLimit = 21000
			}
		} else {
			gasLimit = estimated
		}
	}

	tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, data)
	rawTxBytes, err := tx.MarshalBinary()
	if err != nil {
		return &pb.BuildTransactionResp{Success: false, Message: fmt.Sprintf("failed to marshal tx: %v", err)}, nil
	}
	estimatedFee := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit))

	return &pb.BuildTransactionResp{
		Success:        true,
		Message:        "Transaction built successfully",
		RawTransaction: hexutil.Encode(rawTxBytes),
		Nonce:          nonce,
		GasLimit:       gasLimit,
		GasPrice:       gasPrice.String(),
		EstimatedFee:   estimatedFee.String(),
	}, nil
}
