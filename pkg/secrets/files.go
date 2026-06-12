package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultSecretsMountDir = "/var/run/secrets/internal-wallet"

func readSecretFile(dir string, name string) (string, error) {
	p := filepath.Join(dir, name)
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read %s: %w", p, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func readSecretFileInt(dir string, name string) (int, error) {
	v, err := readSecretFile(dir, name)
	if err != nil {
		return 0, err
	}
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("parse int %s/%s: %w", dir, name, err)
	}
	return n, nil
}

func readSecretFileBool(dir string, name string) (bool, error) {
	v, err := readSecretFile(dir, name)
	if err != nil {
		return false, err
	}
	if v == "" {
		return false, nil
	}
	return v == "true" || v == "1", nil
}

func loadFromFiles(dir string) (*SecretConfig, error) {
	if strings.TrimSpace(dir) == "" {
		dir = defaultSecretsMountDir
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("secrets mount dir not available (%s): %w", dir, err)
	}

	mysqlPassword, err := readSecretFile(dir, "mysql_password")
	if err != nil {
		return nil, err
	}
	redisPassword, err := readSecretFile(dir, "redis_password")
	if err != nil {
		return nil, err
	}

	jwtAccessSecret, err := readSecretFile(dir, "jwt_access_secret")
	if err != nil {
		return nil, err
	}
	jwtRefreshSecret, err := readSecretFile(dir, "jwt_refresh_secret")
	if err != nil {
		return nil, err
	}

	adminJWTAccessSecret, err := readSecretFile(dir, "admin_jwt_access_secret")
	if err != nil {
		return nil, err
	}
	adminJWTRefreshSecret, err := readSecretFile(dir, "admin_jwt_refresh_secret")
	if err != nil {
		return nil, err
	}

	signerEncryptionPassword, err := readSecretFile(dir, "signer_encryption_password")
	if err != nil {
		return nil, err
	}

	smtpHost, err := readSecretFile(dir, "smtp_host")
	if err != nil {
		return nil, err
	}
	if smtpHost == "" {
		smtpHost = getEnv("SMTP_HOST", "")
	}

	smtpPort, err := readSecretFileInt(dir, "smtp_port")
	if err != nil {
		return nil, err
	}
	if smtpPort == 0 {
		smtpPort = getEnvInt("SMTP_PORT", 0)
	}

	smtpUsername, err := readSecretFile(dir, "smtp_username")
	if err != nil {
		return nil, err
	}
	if smtpUsername == "" {
		smtpUsername = getEnv("SMTP_USERNAME", "")
	}

	smtpPassword, err := readSecretFile(dir, "smtp_password")
	if err != nil {
		return nil, err
	}
	if smtpPassword == "" {
		smtpPassword = getEnv("SMTP_PASSWORD", "")
	}

	smtpFromAddress, err := readSecretFile(dir, "smtp_from_address")
	if err != nil {
		return nil, err
	}
	if smtpFromAddress == "" {
		smtpFromAddress = getEnv("SMTP_FROM_ADDRESS", "")
	}

	smtpFromName, err := readSecretFile(dir, "smtp_from_name")
	if err != nil {
		return nil, err
	}
	if smtpFromName == "" {
		smtpFromName = getEnv("SMTP_FROM_NAME", "")
	}

	// Load SWAP secrets
	swapApiKeyPepper, err := readSecretFile(dir, "swap_api_key_pepper")
	if err != nil {
		return nil, err
	}
	swapAdminToken, err := readSecretFile(dir, "swap_admin_token")
	if err != nil {
		return nil, err
	}
	swapAdminTokenSHA256, err := readSecretFile(dir, "swap_admin_token_sha256")
	if err != nil {
		return nil, err
	}
	swapProviderType, err := readSecretFile(dir, "swap_provider_type")
	if err != nil {
		return nil, err
	}
	okxApiKey, err := readSecretFile(dir, "okx_api_key")
	if err != nil {
		return nil, err
	}
	okxSecretKey, err := readSecretFile(dir, "okx_secret_key")
	if err != nil {
		return nil, err
	}
	okxPassphrase, err := readSecretFile(dir, "okx_passphrase")
	if err != nil {
		return nil, err
	}
	oneInchApiKey, err := readSecretFile(dir, "oneinch_api_key")
	if err != nil {
		return nil, err
	}
	businessSwapApiKey, err := readSecretFile(dir, "business_swap_api_key")
	if err != nil {
		return nil, err
	}
	adminSwapAdminToken, err := readSecretFile(dir, "admin_swap_admin_token")
	if err != nil {
		return nil, err
	}

	// Load SMS secrets
	smsInternationalURL, err := readSecretFile(dir, "sms_international_url")
	if err != nil {
		return nil, err
	}
	smsInternationalAppKey, err := readSecretFile(dir, "sms_international_app_key")
	if err != nil {
		return nil, err
	}
	smsInternationalAppSecret, err := readSecretFile(dir, "sms_international_app_secret")
	if err != nil {
		return nil, err
	}
	smsInternationalAppCode, err := readSecretFile(dir, "sms_international_app_code")
	if err != nil {
		return nil, err
	}
	smsDomesticURL, err := readSecretFile(dir, "sms_domestic_url")
	if err != nil {
		return nil, err
	}
	smsDomesticMercId, err := readSecretFile(dir, "sms_domestic_merc_id")
	if err != nil {
		return nil, err
	}
	smsDomesticSecret, err := readSecretFile(dir, "sms_domestic_secret")
	if err != nil {
		return nil, err
	}
	smsDomesticTemplateId, err := readSecretFile(dir, "sms_domestic_template_id")
	if err != nil {
		return nil, err
	}
	smsSign, err := readSecretFile(dir, "sms_sign")
	if err != nil {
		return nil, err
	}

	// Load S3 config secrets (not credentials - using IRSA)
	s3Region, err := readSecretFile(dir, "s3_region")
	if err != nil {
		return nil, err
	}
	s3Bucket, err := readSecretFile(dir, "s3_bucket")
	if err != nil {
		return nil, err
	}
	s3Endpoint, err := readSecretFile(dir, "s3_endpoint")
	if err != nil {
		return nil, err
	}
	s3PublicBaseURL, err := readSecretFile(dir, "s3_public_base_url")
	if err != nil {
		return nil, err
	}
	s3Prefix, err := readSecretFile(dir, "s3_prefix")
	if err != nil {
		return nil, err
	}
	s3UsePathStyle, err := readSecretFileBool(dir, "s3_use_path_style")
	if err != nil {
		return nil, err
	}
	s3ACL, err := readSecretFile(dir, "s3_acl")
	if err != nil {
		return nil, err
	}

	// Load Geetest secrets
	geetestCaptchaID, err := readSecretFile(dir, "geetest_captcha_id")
	if err != nil {
		return nil, err
	}
	geetestCaptchaKey, err := readSecretFile(dir, "geetest_captcha_key")
	if err != nil {
		return nil, err
	}
	geetestAPIServer, err := readSecretFile(dir, "geetest_api_server")
	if err != nil {
		return nil, err
	}
	geetestTimeoutMs, err := readSecretFileInt(dir, "geetest_timeout_ms")
	if err != nil {
		return nil, err
	}
	geetestEnabled, err := readSecretFileBool(dir, "geetest_enabled")
	if err != nil {
		return nil, err
	}
	geetestFailOpen, err := readSecretFileBool(dir, "geetest_fail_open")
	if err != nil {
		return nil, err
	}

	// Load ChainRPC secrets (optional)
	chainrpcEthRpcApiKey, err := readSecretFile(dir, "chainrpc_eth_rpc_api_key")
	if err != nil {
		return nil, err
	}
	chainrpcBscRpcApiKey, err := readSecretFile(dir, "chainrpc_bsc_rpc_api_key")
	if err != nil {
		return nil, err
	}
	chainrpcTronApiKey, err := readSecretFile(dir, "chainrpc_tron_api_key")
	if err != nil {
		return nil, err
	}

	// Load ChainSync secrets (optional)
	chainsyncEthereumApiKey, err := readSecretFile(dir, "chainsync_ethereum_api_key")
	if err != nil {
		return nil, err
	}
	chainsyncBscApiKey, err := readSecretFile(dir, "chainsync_bsc_api_key")
	if err != nil {
		return nil, err
	}
	chainsyncTronApiKey, err := readSecretFile(dir, "chainsync_tron_api_key")
	if err != nil {
		return nil, err
	}
	chainsyncQuickNodeApiKey, err := readSecretFile(dir, "chainsync_quicknode_api_key")
	if err != nil {
		return nil, err
	}
	chainsyncInfuraApiKey, err := readSecretFile(dir, "chainsync_infura_api_key")
	if err != nil {
		return nil, err
	}
	chainsyncAlchemyApiKey, err := readSecretFile(dir, "chainsync_alchemy_api_key")
	if err != nil {
		return nil, err
	}
	chainsyncSelfHostedApiKey, err := readSecretFile(dir, "chainsync_selfhosted_api_key")
	if err != nil {
		return nil, err
	}

	// Load Consolidation iTRX secrets (optional)
	consolidationItrxApiEndpoint, err := readSecretFile(dir, "consolidation_itrx_api_endpoint")
	if err != nil {
		return nil, err
	}
	consolidationItrxApiKey, err := readSecretFile(dir, "consolidation_itrx_api_key")
	if err != nil {
		return nil, err
	}
	consolidationItrxApiSecret, err := readSecretFile(dir, "consolidation_itrx_api_secret")
	if err != nil {
		return nil, err
	}

	return &SecretConfig{
		Version:     "secrets-store-csi",
		Environment: getEnv("APP_ENV", "production"),
		LastUpdated: time.Now(),

		Database: DatabaseConfig{
			MySQL: MySQLConfig{
				Host:           getEnv("MYSQL_HOST", ""),
				Port:           getEnvInt("MYSQL_PORT", 0),
				Username:       getEnv("MYSQL_USERNAME", ""),
				Password:       mysqlPassword,
				Database:       getEnv("MYSQL_DATABASE", ""),
				MaxConnections: getEnvInt("MYSQL_MAX_CONNECTIONS", 0),
			},
		},

		Cache: CacheConfig{
			Redis: RedisConfig{
				Host:     getEnv("REDIS_HOST", ""),
				Port:     getEnvInt("REDIS_PORT", 0),
				Password: redisPassword,
				DB:       getEnvInt("REDIS_DB", 0),
			},
		},

		Messaging: MessagingConfig{
			Kafka: KafkaConfig{
				Brokers:  getEnvSlice("KAFKA_BROKERS", nil),
				Username: getEnv("KAFKA_USERNAME", ""),
				Password: getEnv("KAFKA_PASSWORD", ""),
			},
		},

		Security: SecurityConfig{
			Signer: SignerConfig{
				EncryptionPassword: signerEncryptionPassword,
			},
			JWT: JWTConfig{
				AccessSecret:        jwtAccessSecret,
				RefreshSecret:       jwtRefreshSecret,
				AccessExpireMinutes: getEnvInt("JWT_ACCESS_EXPIRE_MINUTES", 0),
				RefreshExpireDays:   getEnvInt("JWT_REFRESH_EXPIRE_DAYS", 0),
			},
			AdminJWT: JWTConfig{
				AccessSecret:        adminJWTAccessSecret,
				RefreshSecret:       adminJWTRefreshSecret,
				AccessExpireMinutes: getEnvInt("ADMIN_JWT_ACCESS_EXPIRE_MINUTES", 0),
				RefreshExpireDays:   getEnvInt("ADMIN_JWT_REFRESH_EXPIRE_DAYS", 0),
			},
		},

		Notifications: NotificationConfig{
			SMTP: SMTPConfig{
				Host:        smtpHost,
				Port:        smtpPort,
				Username:    smtpUsername,
				Password:    smtpPassword,
				FromAddress: smtpFromAddress,
				FromName:    smtpFromName,
			},
			JPush: JPushConfig{
				AppKey:       getEnv("JPUSH_APP_KEY", ""),
				MasterSecret: getEnv("JPUSH_MASTER_SECRET", ""),
			},
		},

		Swap: SwapConfig{
			ApiKeyPepper:        swapApiKeyPepper,
			AdminToken:          swapAdminToken,
			AdminTokenSHA256:    swapAdminTokenSHA256,
			ProviderType:        swapProviderType,
			OKXApiKey:           okxApiKey,
			OKXSecretKey:        okxSecretKey,
			OKXPassphrase:       okxPassphrase,
			OneInchApiKey:       oneInchApiKey,
			BusinessSwapApiKey:  businessSwapApiKey,
			AdminSwapAdminToken: adminSwapAdminToken,
		},

		SMS: SMSSecretConfig{
			InternationalURL:       smsInternationalURL,
			InternationalAppKey:    smsInternationalAppKey,
			InternationalAppSecret: smsInternationalAppSecret,
			InternationalAppCode:   smsInternationalAppCode,
			DomesticURL:            smsDomesticURL,
			DomesticMercId:         smsDomesticMercId,
			DomesticSecret:         smsDomesticSecret,
			DomesticTemplateId:     smsDomesticTemplateId,
			Sign:                   smsSign,
		},

		S3: S3ConfigSecrets{
			Region:        s3Region,
			Bucket:        s3Bucket,
			Endpoint:      s3Endpoint,
			PublicBaseURL: s3PublicBaseURL,
			Prefix:        s3Prefix,
			UsePathStyle:  s3UsePathStyle,
			ACL:           s3ACL,
		},
		Geetest: GeetestSecretConfig{
			Enabled:    geetestEnabled,
			CaptchaID:  geetestCaptchaID,
			CaptchaKey: geetestCaptchaKey,
			APIServer:  geetestAPIServer,
			Timeout:    int64(geetestTimeoutMs),
			FailOpen:   geetestFailOpen,
		},

		ChainRPC: ChainRPCSecretConfig{
			EthRpcApiKey: chainrpcEthRpcApiKey,
			BscRpcApiKey: chainrpcBscRpcApiKey,
			TronApiKey:   chainrpcTronApiKey,
		},
		ChainSync: ChainSyncSecretConfig{
			EthereumApiKey:   chainsyncEthereumApiKey,
			BscApiKey:        chainsyncBscApiKey,
			TronApiKey:       chainsyncTronApiKey,
			QuickNodeApiKey:  chainsyncQuickNodeApiKey,
			InfuraApiKey:     chainsyncInfuraApiKey,
			AlchemyApiKey:    chainsyncAlchemyApiKey,
			SelfHostedApiKey: chainsyncSelfHostedApiKey,
		},
		Consolidation: ConsolidationSecretConfig{
			ITRXApiEndpoint: consolidationItrxApiEndpoint,
			ITRXApiKey:      consolidationItrxApiKey,
			ITRXApiSecret:   consolidationItrxApiSecret,
		},
	}, nil
}
