package storage

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"internalwallet/api-gateway/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

type S3Uploader struct {
	client        *s3.Client
	bucket        string
	region        string
	publicBaseURL string
	prefix        string
	acl           string
}

func NewS3Uploader(ctx context.Context, cfg config.S3Config) (*S3Uploader, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("s3 uploader disabled")
	}

	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	region := strings.TrimSpace(cfg.Region)

	loadOpts := make([]func(*awsconfig.LoadOptions) error, 0, 3)
	if region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(region))
	}
	if strings.TrimSpace(cfg.AccessKeyID) != "" && strings.TrimSpace(cfg.SecretAccessKey) != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				strings.TrimSpace(cfg.AccessKeyID),
				strings.TrimSpace(cfg.SecretAccessKey),
				strings.TrimSpace(cfg.SessionToken),
			),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, err
	}

	if endpoint != "" {
		awsCfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			if service == s3.ServiceID {
				return aws.Endpoint{
					URL:               endpoint,
					HostnameImmutable: true,
				}, nil
			}
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		})
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	publicBaseURL := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	if publicBaseURL == "" {
		if region == "" {
			publicBaseURL = fmt.Sprintf("https://%s.s3.amazonaws.com", bucket)
		} else {
			publicBaseURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucket, region)
		}
	}

	prefix := strings.Trim(strings.TrimSpace(cfg.Prefix), "/")

	return &S3Uploader{
		client:        client,
		bucket:        bucket,
		region:        region,
		publicBaseURL: publicBaseURL,
		prefix:        prefix,
		acl:           strings.TrimSpace(cfg.ACL),
	}, nil
}

func (u *S3Uploader) CurrencyIconObjectKey(assetCode string, ext string) string {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	parts := make([]string, 0, 5)
	if u.prefix != "" {
		parts = append(parts, u.prefix)
	}
	parts = append(parts, "currencies", "icons")
	if assetCode != "" {
		parts = append(parts, assetCode)
	}
	parts = append(parts, uuid.NewString()+ext)
	return path.Join(parts...)
}

func (u *S3Uploader) ChainIconObjectKey(chainCode string, ext string) string {
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	parts := make([]string, 0, 5)
	if u.prefix != "" {
		parts = append(parts, u.prefix)
	}
	// NOTE: Keep chain icons under the same public directory as currency icons.
	// Some deployments only whitelist public access for `.../currencies/icons/**`.
	parts = append(parts, "currencies", "icons", "chains")
	if chainCode != "" {
		parts = append(parts, chainCode)
	}
	parts = append(parts, uuid.NewString()+ext)
	return path.Join(parts...)
}

func (u *S3Uploader) UserAvatarObjectKey(userID int64, ext string) string {
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	parts := make([]string, 0, 8)
	if u.prefix != "" {
		parts = append(parts, u.prefix)
	}
	// NOTE: Keep user avatars under the same public directory as currency icons.
	// Some deployments only whitelist public access for `.../currencies/icons/**`.
	parts = append(parts, "currencies", "icons", "avatars", "users", strconv.FormatInt(userID, 10), uuid.NewString()+ext)
	return path.Join(parts...)
}

func (u *S3Uploader) DepositQRCodeObjectKey(assetCode string, chainCode string, addressHash string, ext string) string {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	chainCode = strings.ToUpper(strings.TrimSpace(chainCode))
	addressHash = strings.TrimSpace(addressHash)
	ext = strings.TrimSpace(ext)
	if ext == "" {
		ext = ".png"
	} else if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	parts := make([]string, 0, 8)
	if u.prefix != "" {
		parts = append(parts, u.prefix)
	}
	// NOTE: Keep deposit QR codes under the same public directory as currency icons.
	// Some deployments only whitelist public access for `.../currencies/icons/**`.
	parts = append(parts, "currencies", "icons", "qrcode", "deposit")
	if assetCode != "" {
		parts = append(parts, assetCode)
	}
	if chainCode != "" {
		parts = append(parts, chainCode)
	}

	filename := addressHash
	if filename == "" {
		filename = uuid.NewString()
	}
	parts = append(parts, filename+ext)

	return path.Join(parts...)
}

func (u *S3Uploader) Upload(ctx context.Context, objectKey string, contentType string, data []byte) (string, error) {
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if objectKey == "" {
		return "", fmt.Errorf("object key is required")
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	in := &s3.PutObjectInput{
		Bucket:       aws.String(u.bucket),
		Key:          aws.String(objectKey),
		Body:         bytes.NewReader(data),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	}
	if u.acl != "" {
		in.ACL = types.ObjectCannedACL(u.acl)
	}

	if _, err := u.client.PutObject(ctx, in); err != nil {
		return "", err
	}

	return u.publicURL(objectKey), nil
}

func (u *S3Uploader) publicURL(objectKey string) string {
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	return u.publicBaseURL + "/" + objectKey
}
