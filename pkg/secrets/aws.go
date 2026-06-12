package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

// loadFromAWS 从 AWS Secrets Manager 加载配置
func loadFromAWS(ctx context.Context, env string) (*SecretConfig, error) {
	start := time.Now()

	// 1. 检查必需的环境变量
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1" // 默认区域
		fmt.Printf("⚠️  AWS_REGION not set, using default: %s\n", region)
	}

	// 2. 创建 AWS Session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	// 3. 创建 Secrets Manager 客户端
	svc := secretsmanager.New(sess)

	// 4. 构造 Secret 名称（格式：crypto-wallet/{env}/config）
	secretName := fmt.Sprintf("crypto-wallet/%s/config", env)
	fmt.Printf("📦 Fetching secret: %s from region: %s\n", secretName, region)

	// 5. 获取 Secret
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := svc.GetSecretValueWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	// 6. 检查返回结果
	if result.SecretString == nil {
		return nil, fmt.Errorf("secret %s has no SecretString (might be binary)", secretName)
	}

	// 7. 解析 JSON
	var config SecretConfig
	if err := json.Unmarshal([]byte(*result.SecretString), &config); err != nil {
		return nil, fmt.Errorf("failed to parse secret JSON: %w", err)
	}

	// 8. 记录成功日志（不记录实际密钥值）
	elapsed := time.Since(start)
	fmt.Printf("✅ [SECRETS] Loaded config from AWS in %v\n", elapsed)
	fmt.Printf("   - Version: %s\n", config.Version)
	fmt.Printf("   - Environment: %s\n", config.Environment)
	fmt.Printf("   - LastUpdated: %s\n", config.LastUpdated.Format(time.RFC3339))
	fmt.Printf("   - VersionId: %s\n", *result.VersionId)

	return &config, nil
}
