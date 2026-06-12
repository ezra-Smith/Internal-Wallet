package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// findProjectRoot 向上查找项目根目录（包含 go.mod 的目录）
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// 最多向上查找 10 层
	for i := 0; i < 10; i++ {
		// 检查是否存在 go.mod 或 .env 文件
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
			return dir, nil
		}

		// 向上一层
		parent := filepath.Dir(dir)
		if parent == dir {
			// 已到根目录
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("project root not found (no go.mod or .env file)")
}

// loadFromLocal 从本地 .env 文件加载配置
func loadFromLocal() (*SecretConfig, error) {
	// 1. 获取当前工作目录（用于调试）
	cwd, _ := os.Getwd()
	fmt.Printf("🔍 Current working directory: %s\n", cwd)

	// 2. 查找项目根目录
	projectRoot, err := findProjectRoot()
	if err != nil {
		fmt.Printf("⚠️  Cannot find project root: %v\n", err)
		fmt.Printf("ℹ️  Using environment variables only\n")
	} else {
		fmt.Printf("📁 Project root found: %s\n", projectRoot)
		envPath := filepath.Join(projectRoot, ".env")

		// 3. 检查 .env 文件是否存在
		if info, err := os.Stat(envPath); err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("❌ .env file does not exist at %s\n", envPath)
			} else if os.IsPermission(err) {
				fmt.Printf("❌ .env file permission denied at %s\n", envPath)
			} else {
				fmt.Printf("❌ .env file stat error: %v\n", err)
			}
			fmt.Printf("ℹ️  Using environment variables only\n")
		} else {
			fmt.Printf("📄 .env file found: %s (size: %d bytes, mode: %s)\n",
				envPath, info.Size(), info.Mode())

			// 4. 尝试加载 .env 文件
			if err := godotenv.Load(envPath); err != nil {
				fmt.Printf("❌ Failed to load .env file: %v\n", err)
				fmt.Printf("ℹ️  Using environment variables only\n")
			} else {
				fmt.Printf("✅ .env file loaded successfully from %s\n", envPath)
				// 显示已加载的关键变量（不显示敏感值）
				showLoadedKeys := []string{
					"MYSQL_HOST", "MYSQL_PORT", "REDIS_HOST", "REDIS_PORT",
					"KAFKA_BROKERS", "APP_ENV",
				}
				fmt.Printf("📊 Loaded environment variables:\n")
				for _, key := range showLoadedKeys {
					if value := os.Getenv(key); value != "" {
						// 截断长值用于显示
						displayValue := value
						if len(displayValue) > 50 {
							displayValue = displayValue[:47] + "..."
						}
						fmt.Printf("   - %s=%s\n", key, displayValue)
					}
				}
			}
		}
	}

	// 5. 构建配置
	config := &SecretConfig{
		Version:     "local",
		Environment: getEnv("APP_ENV", "local"),
		LastUpdated: time.Now(),

		Database: DatabaseConfig{
			MySQL: MySQLConfig{
				Host:           getEnv("MYSQL_HOST", "localhost"),
				Port:           getEnvInt("MYSQL_PORT", 15306),
				Username:       getEnv("MYSQL_USERNAME", "crypto"),
				Password:       getEnv("MYSQL_PASSWORD", ""),
				Database:       getEnv("MYSQL_DATABASE", "crypto_wallet"),
				MaxConnections: getEnvInt("MYSQL_MAX_CONNECTIONS", 10),
			},
		},

		Cache: CacheConfig{
			Redis: RedisConfig{
				Host:     getEnv("REDIS_HOST", "localhost"),
				Port:     getEnvInt("REDIS_PORT", 6379),
				Password: getEnv("REDIS_PASSWORD", ""),
				DB:       getEnvInt("REDIS_DB", 0),
			},
		},

		Messaging: MessagingConfig{
			Kafka: KafkaConfig{
				Brokers:  getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
				Username: getEnv("KAFKA_USERNAME", ""),
				Password: getEnv("KAFKA_PASSWORD", ""),
			},
		},

		Security: SecurityConfig{
			Signer: SignerConfig{
				EncryptionPassword: getEnv("SIGNER_ENCRYPTION_PASSWORD", ""),
			},
			JWT: JWTConfig{
				AccessSecret:        getEnv("JWT_ACCESS_SECRET", ""),
				RefreshSecret:       getEnv("JWT_REFRESH_SECRET", ""),
				AccessExpireMinutes: getEnvInt("JWT_ACCESS_EXPIRE_MINUTES", 60),
				RefreshExpireDays:   getEnvInt("JWT_REFRESH_EXPIRE_DAYS", 7),
			},
			AdminJWT: JWTConfig{
				AccessSecret:        getEnv("ADMIN_JWT_ACCESS_SECRET", ""),
				RefreshSecret:       getEnv("ADMIN_JWT_REFRESH_SECRET", ""),
				AccessExpireMinutes: getEnvInt("ADMIN_JWT_ACCESS_EXPIRE_MINUTES", 30),
				RefreshExpireDays:   getEnvInt("ADMIN_JWT_REFRESH_EXPIRE_DAYS", 1),
			},
		},

		Notifications: NotificationConfig{
			SMTP: SMTPConfig{
				Host:        getEnv("SMTP_HOST", getEnv("EMAIL_SMTP_HOST", "smtp.gmail.com")),
				Port:        getEnvInt("SMTP_PORT", getEnvInt("EMAIL_SMTP_PORT", 587)), // 0 表示未设置，使用 business.yaml 中的默认值
				Username:    getEnv("SMTP_USERNAME", getEnv("EMAIL_FROM_ADDRESS", "")),
				Password:    getEnv("SMTP_PASSWORD", getEnv("EMAIL_FROM_PASSWORD", "")),
				FromAddress: getEnv("SMTP_FROM_ADDRESS", getEnv("EMAIL_FROM_ADDRESS", "zinkfinance@gmail.com")),
				FromName:    getEnv("SMTP_FROM_NAME", getEnv("EMAIL_FROM_NAME", "Zink Wallet")),
			},
		},

		Monitoring: MonitoringConfig{
			GrafanaAdminPassword: getEnv("GRAFANA_ADMIN_PASSWORD", "admin123456"),
		},

		Swap: SwapConfig{
			ApiKeyPepper:        getEnv("SWAP_API_KEY_PEPPER", ""),
			AdminToken:          getEnv("SWAP_ADMIN_TOKEN", ""),
			AdminTokenSHA256:    getEnv("SWAP_ADMIN_TOKEN_SHA256", ""),
			ProviderType:        getEnv("SWAP_PROVIDER_TYPE", "okx"),
			OKXApiKey:           getEnv("OKX_API_KEY", ""),
			OKXSecretKey:        getEnv("OKX_SECRET_KEY", ""),
			OKXPassphrase:       getEnv("OKX_PASSPHRASE", ""),
			OneInchApiKey:       getEnv("ONEINCH_API_KEY", ""),
			BusinessSwapApiKey:  getEnv("BUSINESS_SWAP_API_KEY", ""),
			AdminSwapAdminToken: getEnv("ADMIN_SWAP_ADMIN_TOKEN", ""),
		},

		SMS: SMSSecretConfig{
			InternationalURL:       getEnv("SMS_INTERNATIONAL_URL", "http://47.242.85.7:9090/sms/batch/v2"),
			InternationalAppKey:    getEnv("SMS_INTERNATIONAL_APP_KEY", "Damtta"),
			InternationalAppSecret: getEnv("SMS_INTERNATIONAL_APP_SECRET", "xqBmnh"),
			InternationalAppCode:   getEnv("SMS_INTERNATIONAL_APP_CODE", "1000"),
			DomesticURL:            getEnv("SMS_DOMESTIC_URL", "https://pla26.1f8rm.cc/api/sms/sendSms"),
			DomesticMercId:         getEnv("SMS_DOMESTIC_MERC_ID", "10240"),
			DomesticSecret:         getEnv("SMS_DOMESTIC_SECRET", "dafc04ff5a07415d048e3f53e0ef3127"),
			DomesticTemplateId:     getEnv("SMS_DOMESTIC_TEMPLATE_ID", "64005daa4d0d6c39420dc226"),
			Sign:                   getEnv("SMS_SIGN", "【Zink】"),
		},

		S3: S3ConfigSecrets{
			Region:        getEnv("S3_REGION", ""),
			Bucket:        getEnv("S3_BUCKET", ""),
			Endpoint:      getEnv("S3_ENDPOINT", ""),
			PublicBaseURL: getEnv("S3_PUBLIC_BASE_URL", ""),
			Prefix:        getEnv("S3_PREFIX", "admin"),
			UsePathStyle:  getEnv("S3_USE_PATH_STYLE", "false") == "true",
			ACL:           getEnv("S3_ACL", ""),
		},

		Geetest: GeetestSecretConfig{
			Enabled:    getEnvBool("GEETEST_ENABLED", false),
			CaptchaID:  getEnv("GEETEST_CAPTCHA_ID", ""),
			CaptchaKey: getEnv("GEETEST_CAPTCHA_KEY", ""),
			APIServer:  getEnv("GEETEST_API_SERVER", ""),
			Timeout:    int64(getEnvInt("GEETEST_TIMEOUT_MS", 0)),
			FailOpen:   getEnvBool("GEETEST_FAIL_OPEN", true),
		},
	}

	return config, nil
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt 获取整数类型的环境变量
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return defaultValue
}

// getEnvSlice 获取数组类型的环境变量（逗号分隔）
func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
