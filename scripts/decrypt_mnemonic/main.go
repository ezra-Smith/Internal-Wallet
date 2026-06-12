package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/pbkdf2"
)

// 数据库配置
const (
	dbHost     = "localhost"
	dbPort     = 15306
	dbUser     = "root"
	dbPassword = "root123456"
	dbName     = "crypto_wallet"
)

// 解密密码（初始化热钱包时使用的 unlock_password）
const decryptPassword = "MyVerySecureEncryptionPassword2024!@#$%"

// 要解密的 seed_id
const targetSeedID = "hot_wallet_main"

func main() {
	// 连接数据库
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("数据库连接测试失败: %v", err)
	}
	fmt.Println("✅ 数据库连接成功")
	fmt.Println()

	// 0. 先查找所有可能包含助记词的表
	fmt.Println("========== 查找助记词相关表 ==========")
	findMnemonicTables(db)
	fmt.Println()

	// 1. 尝试从 wallet_master_mnemonics 表解密助记词
	fmt.Println("========== 解密助记词 (wallet_master_mnemonics) ==========")
	decryptMnemonicFromDB(db, targetSeedID)

	fmt.Println()

	// 2. 尝试从 master_seeds 表解密 BIP39 Seed
	fmt.Println("========== 解密 BIP39 Seed (master_seeds) ==========")
	decryptSeedFromDB(db, targetSeedID)

	fmt.Println()

	// 3. 尝试从 mnemonic_backups 表解密
	fmt.Println("========== 解密助记词备份 (mnemonic_backups) ==========")
	decryptMnemonicBackup(db, targetSeedID)
}

// findMnemonicTables 查找可能包含助记词的表
func findMnemonicTables(db *sql.DB) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		fmt.Printf("❌ 查询表失败: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("可能包含助记词/种子的表：")
	for rows.Next() {
		var tableName string
		rows.Scan(&tableName)
		// 筛选可能相关的表
		if contains(tableName, "mnemonic") || contains(tableName, "seed") || contains(tableName, "wallet") || contains(tableName, "backup") {
			// 查看表结构
			fmt.Printf("  📁 %s\n", tableName)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// decryptMnemonicBackup 从 mnemonic_backups 表解密
func decryptMnemonicBackup(db *sql.DB, seedID string) {
	// 先查看表结构
	fmt.Println("📋 mnemonic_backups 表结构:")
	rows, err := db.Query("DESCRIBE mnemonic_backups")
	if err != nil {
		fmt.Printf("❌ 查询表结构失败: %v\n", err)
		return
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var field, typ, null, key string
		var def, extra sql.NullString
		rows.Scan(&field, &typ, &null, &key, &def, &extra)
		columns = append(columns, field)
		fmt.Printf("   - %s (%s)\n", field, typ)
	}
	fmt.Println()

	// 查看所有数据
	fmt.Println("📋 mnemonic_backups 表数据:")
	dataRows, err := db.Query("SELECT * FROM mnemonic_backups LIMIT 5")
	if err != nil {
		fmt.Printf("❌ 查询数据失败: %v\n", err)
		return
	}
	defer dataRows.Close()

	cols, _ := dataRows.Columns()
	fmt.Printf("   列: %v\n", cols)

	for dataRows.Next() {
		// 动态扫描所有列
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		dataRows.Scan(valuePtrs...)

		fmt.Printf("   行: ")
		for i, col := range cols {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				if len(v) > 20 {
					fmt.Printf("%s=[%d bytes] ", col, len(v))
				} else {
					fmt.Printf("%s=%s ", col, string(v))
				}
			case nil:
				fmt.Printf("%s=NULL ", col)
			default:
				fmt.Printf("%s=%v ", col, v)
			}
		}
		fmt.Println()
	}

	// 尝试按 seed_id 查询
	fmt.Println()
	fmt.Printf("📋 尝试查询 seed_id='%s' 的记录...\n", seedID)

	var (
		id                int64
		mnemonicEncrypted []byte
		encryptionSalt    []byte
		wordCount         int
	)

	// 直接用 seed_id 查询
	query := `SELECT id, mnemonic_encrypted, encryption_salt, word_count FROM mnemonic_backups WHERE seed_id = ? AND deleted = 0 LIMIT 1`
	err = db.QueryRow(query, seedID).Scan(&id, &mnemonicEncrypted, &encryptionSalt, &wordCount)
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		return
	}

	fmt.Printf("📋 找到备份记录 ID: %d, 词数: %d\n", id, wordCount)
	fmt.Printf("   加密数据长度: %d bytes\n", len(mnemonicEncrypted))
	fmt.Printf("   Salt 长度: %d bytes\n", len(encryptionSalt))

	// 解密
	mnemonic, err := DecryptSeed(mnemonicEncrypted, encryptionSalt, decryptPassword)
	if err != nil {
		fmt.Printf("❌ 解密失败: %v\n", err)
		fmt.Println("   可能原因：密码不正确")
		return
	}

	fmt.Println()
	fmt.Println("✅ ✅ ✅ 解密成功！ ✅ ✅ ✅")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("🔑 助记词 (%d 个单词):\n", wordCount)
	fmt.Println()
	fmt.Printf("   %s\n", string(mnemonic))
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// decryptMnemonicFromDB 从 wallet_master_mnemonics 表解密助记词
func decryptMnemonicFromDB(db *sql.DB, seedID string) {
	var (
		id                int64
		mnemonicEncrypted []byte
		mnemonicSalt      []byte
		mnemonicIV        []byte
		wordCount         int
	)

	query := `SELECT id, mnemonic_encrypted, mnemonic_salt, mnemonic_iv, word_count 
	          FROM wallet_master_mnemonics 
	          WHERE seed_id = ? AND deleted_at IS NULL 
	          LIMIT 1`

	err := db.QueryRow(query, seedID).Scan(&id, &mnemonicEncrypted, &mnemonicSalt, &mnemonicIV, &wordCount)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("❌ 未找到 seed_id='%s' 的助记词记录\n", seedID)
			fmt.Println("   可能原因：热钱包初始化流程未完成，或使用了其他方式存储")
		} else {
			fmt.Printf("❌ 查询失败: %v\n", err)
		}
		return
	}

	fmt.Printf("📋 找到记录 ID: %d, 词数: %d\n", id, wordCount)
	fmt.Printf("   加密数据长度: %d bytes\n", len(mnemonicEncrypted))
	fmt.Printf("   Salt 长度: %d bytes\n", len(mnemonicSalt))

	// 解密
	mnemonic, err := DecryptSeed(mnemonicEncrypted, mnemonicSalt, decryptPassword)
	if err != nil {
		fmt.Printf("❌ 解密失败: %v\n", err)
		fmt.Println("   可能原因：密码不正确")
		return
	}

	fmt.Println()
	fmt.Println("✅ 解密成功！")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("🔑 助记词:\n%s\n", string(mnemonic))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// decryptSeedFromDB 从 master_seeds 表解密 BIP39 Seed
func decryptSeedFromDB(db *sql.DB, seedID string) {
	var (
		id            int64
		seedName      string
		seedEncrypted []byte
		encryptionSalt []byte
		seedHash      string
	)

	query := `SELECT id, seed_name, seed_encrypted, encryption_salt, seed_hash 
	          FROM master_seeds 
	          WHERE seed_id = ? AND deleted_at IS NULL 
	          LIMIT 1`

	err := db.QueryRow(query, seedID).Scan(&id, &seedName, &seedEncrypted, &encryptionSalt, &seedHash)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("❌ 未找到 seed_id='%s' 的种子记录\n", seedID)
		} else {
			fmt.Printf("❌ 查询失败: %v\n", err)
		}
		return
	}

	fmt.Printf("📋 找到记录 ID: %d, 名称: %s\n", id, seedName)
	fmt.Printf("   加密数据长度: %d bytes\n", len(seedEncrypted))
	fmt.Printf("   Salt 长度: %d bytes\n", len(encryptionSalt))
	fmt.Printf("   Seed Hash: %s\n", seedHash)

	// 解密
	seed, err := DecryptSeed(seedEncrypted, encryptionSalt, decryptPassword)
	if err != nil {
		fmt.Printf("❌ 解密失败: %v\n", err)
		fmt.Println("   可能原因：密码不正确")
		return
	}

	fmt.Println()
	fmt.Println("✅ 解密成功！")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("🔑 BIP39 Seed (Hex):\n%s\n", hex.EncodeToString(seed))
	fmt.Printf("   长度: %d bytes\n", len(seed))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 验证 hash
	actualHash := HashSeed(seed)
	if actualHash == seedHash {
		fmt.Println("✅ Hash 验证通过")
	} else {
		fmt.Printf("⚠️  Hash 不匹配！期望: %s, 实际: %s\n", seedHash, actualHash)
	}
}

// DecryptSeed 解密 Master Seed / 助记词
// 使用 AES-256-GCM 解密，PBKDF2 密钥派生
func DecryptSeed(encrypted []byte, salt []byte, password string) ([]byte, error) {
	if len(encrypted) == 0 {
		return nil, fmt.Errorf("加密数据为空")
	}
	if len(salt) == 0 {
		return nil, fmt.Errorf("salt 为空")
	}
	if len(password) == 0 {
		return nil, fmt.Errorf("密码为空")
	}

	// 1. 从密码派生密钥（使用相同的参数：100000 次迭代，32 字节密钥）
	key := pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)

	// 2. 创建 AES-GCM 解密器
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 cipher 失败: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 3. 提取 nonce 和密文
	// 加密格式: nonce(12字节) + 密文 + tag(16字节)
	nonceSize := gcm.NonceSize() // 12 字节
	if len(encrypted) < nonceSize {
		return nil, fmt.Errorf("加密数据太短: %d bytes", len(encrypted))
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]

	// 4. 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败（密码可能不正确）: %w", err)
	}

	return plaintext, nil
}

// HashSeed 计算 Seed 的 SHA256 哈希
func HashSeed(seed []byte) string {
	hash := sha256.Sum256(seed)
	return hex.EncodeToString(hash[:])
}
