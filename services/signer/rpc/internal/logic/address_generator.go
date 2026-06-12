package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/ripemd160"
)

// ChainConfig 链配置
type ChainConfig struct {
	Chain    string // "TRN", "BSC", "ETH"
	CoinType uint32 // BIP44 coin type
	Account  uint32 // 账户索引
}

var (
	// ChainConfigs 链配置映射
	// 注意：BSC 和 ETH 使用相同的 CoinType (60) 和 Account (0)，
	// 这是因为它们共享相同的地址格式（Ethereum 地址），相同的 HD 路径会生成相同的地址。
	// 这与交易所的做法一致：BSC 和 ETH 地址是互通的。
	ChainConfigs = map[string]ChainConfig{
		"TRON": {
			Chain:    "TRON",
			CoinType: 195, // TRON coin type
			Account:  0,
		},
		"BSC": {
			Chain:    "BSC",
			CoinType: 60, // Ethereum coin type (BSC 与 ETH 共享，地址格式相同)
			Account:  0,
		},
		"ETH": {
			Chain:    "ETH",
			CoinType: 60, // Ethereum coin type (与 BSC 共享，地址格式相同)
			Account:  0,
		},
	}
)

// BuildDerivationPath 构建派生路径
// 方案A: 使用user_id作为address_index
// 注意：user_id必须 >= 1000，0-1000保留给公司地址
func BuildDerivationPath(chain string, userID int64) (string, error) {
	return BuildDerivationPathWithChange(chain, userID, 0)
}

// BuildDerivationPathWithChange 构建派生路径（支持 change=subIndex，用于同链多地址）
// BIP44派生路径: m/44'/coin_type'/account'/change/address_index
func BuildDerivationPathWithChange(chain string, userID int64, changeIndex int32) (string, error) {
	config, ok := ChainConfigs[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}

	// 安全检查：防止使用保留的公司地址索引
	if userID < 1000 {
		return "", fmt.Errorf("invalid user_id: %d, user addresses must start from 100001 (0-100 reserved for company)", userID)
	}

	if changeIndex < 0 {
		return "", fmt.Errorf("invalid change_index: %d", changeIndex)
	}

	// m/44'/coin_type'/account'/change/address_index
	path := fmt.Sprintf("m/44'/%d'/%d'/%d/%d", config.CoinType, config.Account, changeIndex, userID)
	return path, nil
}

// BuildCompanyWalletPath 构建公司钱包派生路径
// addressIndex: 1=热钱包主地址, 2=热钱包备用, 3=冷钱包主地址, etc.
func BuildCompanyWalletPath(chain string, addressIndex int64) (string, error) {
	config, ok := ChainConfigs[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}

	// 安全检查：确保使用的是公司地址索引范围
	if addressIndex < 1 || addressIndex >= 1000 {
		return "", fmt.Errorf("invalid company address index: %d, must be 1-100", addressIndex)
	}

	// m/44'/coin_type'/account'/0/address_index
	path := fmt.Sprintf("m/44'/%d'/%d'/0/%d", config.CoinType, config.Account, addressIndex)
	return path, nil
}

// PublicKeyToAddress 将公钥转换为对应链的地址
func PublicKeyToAddress(chain, pubKeyHex string) (string, error) {
	pubKeyBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode public key: %w", err)
	}

	switch chain {
	case "TRON":
		return PublicKeyToTronAddress(pubKeyBytes)
	case "BSC", "ETH":
		return PublicKeyToEthereumAddress(pubKeyBytes)
	default:
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}
}

// PublicKeyToEthereumAddress 将公钥转换为Ethereum/BSC地址
func PublicKeyToEthereumAddress(pubKeyBytes []byte) (string, error) {
	// 如果是压缩公钥(33字节)，需要先解压缩
	if len(pubKeyBytes) == 33 {
		pubKey, err := crypto.DecompressPubkey(pubKeyBytes)
		if err != nil {
			return "", fmt.Errorf("failed to decompress public key: %w", err)
		}
		pubKeyBytes = crypto.FromECDSAPub(pubKey)
	}

	// 移除前缀04 (未压缩公钥格式)
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		pubKeyBytes = pubKeyBytes[1:]
	}

	if len(pubKeyBytes) != 64 {
		return "", fmt.Errorf("invalid public key length: %d", len(pubKeyBytes))
	}

	// Keccak256哈希，取后20字节
	hash := crypto.Keccak256(pubKeyBytes)
	address := common.BytesToAddress(hash[12:])

	return address.Hex(), nil
}

// PublicKeyToTronAddress 将公钥转换为TRON地址
func PublicKeyToTronAddress(pubKeyBytes []byte) (string, error) {
	// 如果是压缩公钥(33字节)，需要先解压缩
	if len(pubKeyBytes) == 33 {
		pubKey, err := crypto.DecompressPubkey(pubKeyBytes)
		if err != nil {
			return "", fmt.Errorf("failed to decompress public key: %w", err)
		}
		pubKeyBytes = crypto.FromECDSAPub(pubKey)
	}

	// 移除前缀04
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		pubKeyBytes = pubKeyBytes[1:]
	}

	if len(pubKeyBytes) != 64 {
		return "", fmt.Errorf("invalid public key length: %d", len(pubKeyBytes))
	}

	// 1. Keccak256哈希，取后20字节
	hash := crypto.Keccak256(pubKeyBytes)
	addressBytes := hash[12:]

	// 2. 添加TRON前缀 0x41
	addressBytes = append([]byte{0x41}, addressBytes...)

	// 3. 双SHA256计算校验和
	checksum := doubleSHA256(addressBytes)

	// 4. 取校验和的前4字节
	addressBytes = append(addressBytes, checksum[:4]...)

	// 5. Base58编码
	address := base58.Encode(addressBytes)

	return address, nil
}

// doubleSHA256 双SHA256哈希
func doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// PublicKeyToTronAddressOld TRON地址生成（备用方法，使用RIPEMD160）
func PublicKeyToTronAddressOld(pubKeyBytes []byte) (string, error) {
	// 移除前缀04
	if len(pubKeyBytes) == 65 && pubKeyBytes[0] == 0x04 {
		pubKeyBytes = pubKeyBytes[1:]
	}

	// SHA256哈希
	sha := sha256.Sum256(pubKeyBytes)

	// RIPEMD160哈希
	ripemd := ripemd160.New()
	ripemd.Write(sha[:])
	hash160 := ripemd.Sum(nil)

	// 添加TRON前缀 0x41
	addressBytes := append([]byte{0x41}, hash160...)

	// 双SHA256计算校验和
	checksum := doubleSHA256(addressBytes)

	// 添加校验和
	addressBytes = append(addressBytes, checksum[:4]...)

	// Base58编码
	address := base58.Encode(addressBytes)

	return address, nil
}

// ValidateChain 验证链是否支持
func ValidateChain(chain string) error {
	if _, ok := ChainConfigs[chain]; !ok {
		return fmt.Errorf("unsupported chain: %s, supported chains: BSC, ETH, TRON", chain)
	}
	return nil
}
