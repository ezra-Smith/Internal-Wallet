package logic

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/fbsobreira/gotron-sdk/pkg/client/transaction"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/fbsobreira/gotron-sdk/pkg/proto/core"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
	"google.golang.org/protobuf/proto"
)

// HDWallet HD钱包辅助工具
type HDWallet struct {
	masterKey *hdkeychain.ExtendedKey
	chain     string
}

// NewHDWalletFromSeed 从Seed创建HD钱包
func NewHDWalletFromSeed(seed []byte, chain string) (*HDWallet, error) {
	if len(seed) < 16 {
		return nil, errors.New("seed too short")
	}

	// 使用btcsuite的hdkeychain创建master key
	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	return &HDWallet{
		masterKey: masterKey,
		chain:     chain,
	}, nil
}

// NewHDWalletFromMnemonic 从助记词创建HD钱包（可选功能）
func NewHDWalletFromMnemonic(mnemonic string, chain string) (*HDWallet, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("invalid mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, "")
	return NewHDWalletFromSeed(seed, chain)
}

// DeriveKey 根据BIP44路径派生密钥
// path格式: m/44'/60'/0'/0/0
func (w *HDWallet) DeriveKey(path string) (*hdkeychain.ExtendedKey, error) {
	if !strings.HasPrefix(path, "m/") {
		return nil, errors.New("invalid path: must start with m/")
	}

	// 解析路径
	pathParts := strings.Split(path[2:], "/")
	if len(pathParts) == 0 {
		return w.masterKey, nil
	}

	key := w.masterKey
	for _, part := range pathParts {
		if part == "" {
			continue
		}

		// 检查是否是硬化派生
		hardened := strings.HasSuffix(part, "'")
		indexStr := strings.TrimSuffix(part, "'")

		var index uint32
		_, err := fmt.Sscanf(indexStr, "%d", &index)
		if err != nil {
			return nil, fmt.Errorf("invalid path component: %s", part)
		}

		if hardened {
			index += hdkeychain.HardenedKeyStart
		}

		key, err = key.Derive(index)
		if err != nil {
			return nil, fmt.Errorf("failed to derive key at %s: %w", part, err)
		}
	}

	return key, nil
}

// GetPublicKey 获取公钥
func (w *HDWallet) GetPublicKey(path string) (uncompressed, compressed string, err error) {
	key, err := w.DeriveKey(path)
	if err != nil {
		return "", "", err
	}

	ecPubKey, err := key.ECPubKey()
	if err != nil {
		return "", "", fmt.Errorf("failed to get EC public key: %w", err)
	}

	// 未压缩公钥（65字节，04开头）
	uncompressedBytes := ecPubKey.SerializeUncompressed()
	uncompressed = hex.EncodeToString(uncompressedBytes)

	// 压缩公钥（33字节，02或03开头）
	compressedBytes := ecPubKey.SerializeCompressed()
	compressed = hex.EncodeToString(compressedBytes)

	return uncompressed, compressed, nil
}

// GetPrivateKey 获取私钥（仅用于签名，不对外暴露）
func (w *HDWallet) GetPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	key, err := w.DeriveKey(path)
	if err != nil {
		return nil, err
	}

	ecPrivKey, err := key.ECPrivKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get EC private key: %w", err)
	}

	// 获取原始私钥字节
	privKeyBytes := ecPrivKey.Serialize()

	// 使用 go-ethereum 的 crypto.ToECDSA 转换，确保 Curve 字段正确
	// 这与 gotron-sdk 使用的 crypto.ToECDSA 一致
	privKey, err := crypto.ToECDSA(privKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to ECDSA private key: %w", err)
	}

	return privKey, nil
}

// SignTransactionForChain 根据链类型签名交易
func (w *HDWallet) SignTransactionForChain(chain string, path string, rawTxData []byte) (signature string, txHash string, err error) {
	switch chain {
	case "ETH", "BSC":
		return w.signEVMTransaction(path, rawTxData)
	case "TRN", "TRON":
		return w.signTronTransaction(path, rawTxData)
	default:
		return "", "", fmt.Errorf("unsupported chain: %s", chain)
	}
}

// signEVMTransaction 签名 EVM 链交易（ETH、BSC）
// 参数：
//   - path: HD 钱包派生路径
//   - rawTxData: RLP 编码的未签名交易
//
// 返回：
//   - signedTxHex: 签名后的完整交易（hex 编码，可以直接广播）
//   - txHash: 交易哈希
//   - error: 错误信息
func (w *HDWallet) signEVMTransaction(path string, rawTxData []byte) (signedTxHex string, txHash string, err error) {
	// 1. 获取私钥
	privKey, err := w.GetPrivateKey(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to get private key: %w", err)
	}

	// 2. 解析未签名交易（RLP 编码）
	//var unsignedTx types.Transaction
	unsignedTx := new(types.Transaction)
	if err := unsignedTx.UnmarshalBinary(rawTxData); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	// 3. 根据链类型确定 ChainID
	// 重要：不依赖交易中的 ChainID，因为 legacy 交易反序列化后可能返回错误的 ChainID
	// 始终根据 w.chain 参数来确定正确的 ChainID
	// 注意：此方法仅处理 EVM 兼容链（ETH、BSC），TRON 使用 signTronTransaction 方法
	var chainID *big.Int
	switch w.chain {
	case "BSC":
		chainID = big.NewInt(56) // BSC 主网
	case "ETH":
		chainID = big.NewInt(1) // ETH 主网
	default:
		chainID = big.NewInt(1) // 默认 ETH 主网
	}
	// 4. 创建签名器（使用 EIP-155，包含 ChainID 防止重放攻击）
	signer := types.NewEIP155Signer(chainID)

	// 5. 签名交易
	signedTx, err := types.SignTx(unsignedTx, signer, privKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// 6. 序列化签名后的交易
	signedTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal signed transaction: %w", err)
	}

	// 7. 计算交易哈希
	txHashStr := signedTx.Hash().Hex()

	// 8. 返回 hex 编码的签名交易
	return hexutil.Encode(signedTxBytes), txHashStr, nil
}

// signTronTransaction 签名 TRON 链交易
// 参数：
//   - path: HD 钱包派生路径
//   - rawTxData: Protobuf 编码的未签名 TRON Transaction
//
// 返回：
//   - signedTxHex: 签名后的完整交易（hex 编码，可以直接广播）
//   - txHash: 交易哈希
//   - error: 错误信息
func (w *HDWallet) signTronTransaction(path string, rawTxData []byte) (signedTxHex string, txHash string, err error) {
	// 1. 获取私钥
	privKey, err := w.GetPrivateKey(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to get private key: %w", err)
	}

	// 2. 解析 Protobuf Transaction
	var tx core.Transaction
	if err := proto.Unmarshal(rawTxData, &tx); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal TRON transaction: %w", err)
	}

	// 3. 使用 gotron-sdk 的 SignTransactionECDSA 方法签名
	signedTx, err := transaction.SignTransactionECDSA(&tx, privKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign TRON transaction: %w", err)
	}

	// 4. 序列化签名后的交易
	signedTxBytes, err := proto.Marshal(signedTx)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal signed transaction: %w", err)
	}

	// 5. 计算交易哈希（对 raw_data 进行哈希）
	rawDataBytes, err := proto.Marshal(signedTx.GetRawData())
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal raw_data for hash: %w", err)
	}
	hash := sha256.Sum256(rawDataBytes)
	txHashHex := hexutil.Encode(hash[:])

	// 6. 返回 hex 编码的签名交易
	return hexutil.Encode(signedTxBytes), txHashHex, nil
}

// SignMessage 签名消息（EVM链）
func (w *HDWallet) SignMessage(path string, message string) (signature string, err error) {
	privKey, err := w.GetPrivateKey(path)
	if err != nil {
		return "", err
	}

	// 计算消息哈希
	messageHash := crypto.Keccak256Hash([]byte(message))

	// 签名
	sig, err := crypto.Sign(messageHash.Bytes(), privKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign message: %w", err)
	}

	return hex.EncodeToString(sig), nil
}

// ========== BIP32辅助函数（备用实现） ==========

// DeriveKeyBIP32 使用go-bip32库派生密钥（备用方法）
func DeriveKeyBIP32(seed []byte, path string) (*bip32.Key, error) {
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to create master key: %w", err)
	}

	if !strings.HasPrefix(path, "m/") {
		return nil, errors.New("invalid path: must start with m/")
	}

	pathParts := strings.Split(path[2:], "/")
	key := masterKey

	for _, part := range pathParts {
		if part == "" {
			continue
		}

		hardened := strings.HasSuffix(part, "'")
		indexStr := strings.TrimSuffix(part, "'")

		var index uint32
		_, err := fmt.Sscanf(indexStr, "%d", &index)
		if err != nil {
			return nil, fmt.Errorf("invalid path component: %s", part)
		}

		if hardened {
			key, err = key.NewChildKey(bip32.FirstHardenedChild + index)
		} else {
			key, err = key.NewChildKey(index)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to derive key at %s: %w", part, err)
		}
	}

	return key, nil
}

// GetPublicKeyFromBIP32 从BIP32密钥获取公钥
func GetPublicKeyFromBIP32(key *bip32.Key) (uncompressed, compressed string) {
	publicKey := key.PublicKey()

	// 压缩公钥（BIP32默认返回）
	compressed = hex.EncodeToString(publicKey.Key)

	// 转换为未压缩公钥
	pubKey, err := btcec.ParsePubKey(publicKey.Key, btcec.S256())
	if err == nil {
		uncompressed = hex.EncodeToString(pubKey.SerializeUncompressed())
	}

	return uncompressed, compressed
}

// ========== 路径构建辅助函数 ==========

// BuildBIP44Path 构建BIP44标准路径
// path格式: m/purpose'/coin_type'/account'/change/address_index
func BuildBIP44Path(coinType, account, change, addressIndex uint32) string {
	return fmt.Sprintf("m/44'/%d'/%d'/%d/%d", coinType, account, change, addressIndex)
}

// ParseDerivationPath 解析派生路径
func ParseDerivationPath(path string) ([]uint32, error) {
	if !strings.HasPrefix(path, "m/") {
		return nil, errors.New("invalid path: must start with m/")
	}

	pathParts := strings.Split(path[2:], "/")
	indices := make([]uint32, 0, len(pathParts))

	for _, part := range pathParts {
		if part == "" {
			continue
		}

		hardened := strings.HasSuffix(part, "'")
		indexStr := strings.TrimSuffix(part, "'")

		var index uint32
		_, err := fmt.Sscanf(indexStr, "%d", &index)
		if err != nil {
			return nil, fmt.Errorf("invalid path component: %s", part)
		}

		if hardened {
			index += hdkeychain.HardenedKeyStart
		}

		indices = append(indices, index)
	}

	return indices, nil
}

// ValidatePath 验证派生路径格式
func ValidatePath(path string) error {
	if !strings.HasPrefix(path, "m/") {
		return errors.New("path must start with m/")
	}

	pathParts := strings.Split(path[2:], "/")
	if len(pathParts) == 0 {
		return errors.New("path is empty")
	}

	for _, part := range pathParts {
		if part == "" {
			continue
		}

		indexStr := strings.TrimSuffix(part, "'")
		var index uint32
		_, err := fmt.Sscanf(indexStr, "%d", &index)
		if err != nil {
			return fmt.Errorf("invalid path component: %s", part)
		}
	}

	return nil
}

// HashData 计算数据的SHA256哈希
func HashData(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}
