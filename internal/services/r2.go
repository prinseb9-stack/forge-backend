// Package services contains shared backend services for FORGE.
//
// S3-compatible storage integration (Backblaze B2, Cloudflare R2, or any
// S3-compatible provider): presigned PUT with content-type restriction,
// presigned GET, and object delete. This file is additive only — it does
// not touch handlers, routes, main.go, Agnes, Firestore, or payments.
package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Environment variable names. Real values live in .env locally and in
// Render environment variables in production. Never commit real values.
//
// Backblaze B2 uses:
//
//	R2_ACCESS_KEY_ID     = keyID
//	R2_SECRET_ACCESS_KEY = applicationKey
//	R2_ENDPOINT          = https://s3.<region>.backblazeb2.com
//	R2_REGION            = <region> (e.g. us-west-004)
//
// Cloudflare R2 uses:
//
//	R2_ACCOUNT_ID        = <account_id>
//	R2_ACCESS_KEY_ID     = <access_key>
//	R2_SECRET_ACCESS_KEY = <secret_key>
//	R2_ENDPOINT          = (optional; derived from AccountID if empty)
//	R2_REGION            = (optional; defaults to "auto")
const (
	EnvR2AccountID       = "R2_ACCOUNT_ID"
	EnvR2AccessKeyID     = "R2_ACCESS_KEY_ID"
	EnvR2SecretAccessKey = "R2_SECRET_ACCESS_KEY"
	EnvR2BucketName      = "R2_BUCKET_NAME"
	EnvR2Endpoint        = "R2_ENDPOINT"
	EnvR2Region          = "R2_REGION"
)

// Allowed content types for Phase 1 (photo editing only).
var allowedPutContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
}

// R2Config holds non-logged configuration for S3-compatible storage.
//
// AccountID is only used by Cloudflare R2 when Endpoint is empty.
// For Backblaze B2, set Endpoint and Region directly and leave AccountID empty.
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	Endpoint        string
	Region          string
}

// R2Service is the control-plane client for storage. It generates
// presigned URLs so large files do NOT pass through Render.
type R2Service struct {
	cfg       R2Config
	s3Client  *s3.Client
	presigner *s3.PresignClient
}

// LoadR2ConfigFromEnv reads storage config from environment. On failure
// it returns the names of missing variables — never their values.
//
// Requires: R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET_NAME.
// Also requires either R2_ENDPOINT (B2) or R2_ACCOUNT_ID (Cloudflare R2).
func LoadR2ConfigFromEnv() (R2Config, error) {
	cfg := R2Config{
		AccountID:       strings.TrimSpace(os.Getenv(EnvR2AccountID)),
		AccessKeyID:     strings.TrimSpace(os.Getenv(EnvR2AccessKeyID)),
		SecretAccessKey: strings.TrimSpace(os.Getenv(EnvR2SecretAccessKey)),
		BucketName:      strings.TrimSpace(os.Getenv(EnvR2BucketName)),
		Endpoint:        strings.TrimSpace(os.Getenv(EnvR2Endpoint)),
		Region:          strings.TrimSpace(os.Getenv(EnvR2Region)),
	}

	var missing []string
	if cfg.AccessKeyID == "" {
		missing = append(missing, EnvR2AccessKeyID)
	}
	if cfg.SecretAccessKey == "" {
		missing = append(missing, EnvR2SecretAccessKey)
	}
	if cfg.BucketName == "" {
		missing = append(missing, EnvR2BucketName)
	}
	// Either an explicit Endpoint (B2) or an AccountID (Cloudflare R2) is required.
	if cfg.Endpoint == "" && cfg.AccountID == "" {
		missing = append(missing, EnvR2Endpoint+" (or "+EnvR2AccountID+")")
	}
	if len(missing) > 0 {
		return R2Config{}, fmt.Errorf("missing required storage environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

// NewR2Service creates a storage service from explicit config. It does
// not perform network calls and does not log secrets.
func NewR2Service(ctx context.Context, cfg R2Config) (*R2Service, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg.AccountID = strings.TrimSpace(cfg.AccountID)
	cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
	cfg.SecretAccessKey = strings.TrimSpace(cfg.SecretAccessKey)
	cfg.BucketName = strings.TrimSpace(cfg.BucketName)
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = strings.TrimSpace(cfg.Region)

	if cfg.AccessKeyID == "" {
		return nil, errors.New("R2Config.AccessKeyID is required")
	}
	if cfg.SecretAccessKey == "" {
		return nil, errors.New("R2Config.SecretAccessKey is required")
	}
	if cfg.BucketName == "" {
		return nil, errors.New("R2Config.BucketName is required")
	}
	if cfg.Endpoint == "" && cfg.AccountID == "" {
		return nil, errors.New("R2Config.Endpoint (or AccountID) is required")
	}

	// Resolve endpoint + region.
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	}
	endpoint = strings.TrimRight(endpoint, "/")

	region := cfg.Region
	if region == "" {
		region = "auto"
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for storage: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		// Backblaze B2 requires path-style addressing. This is safe
		// for Cloudflare R2 as well.
		o.UsePathStyle = true
	})

	presigner := s3.NewPresignClient(s3Client)

	return &R2Service{
		cfg:       cfg,
		s3Client:  s3Client,
		presigner: presigner,
	}, nil
}

func (s *R2Service) sanitizeKey(key string) (string, error) {
	k := strings.TrimSpace(key)
	k = strings.TrimPrefix(k, "/")
	if k == "" {
		return "", errors.New("key must not be empty")
	}
	if strings.Contains(k, "\\") {
		return "", errors.New("key must not contain backslashes")
	}
	if k == "." || k == ".." || strings.HasPrefix(k, "../") || strings.Contains(k, "/../") {
		return "", errors.New("invalid key")
	}
	return k, nil
}

func validateExpiry(expires time.Duration) error {
	if expires <= 0 {
		return errors.New("expires must be greater than 0")
	}
	if expires > 7*24*time.Hour {
		return errors.New("expires must be 7 days or less")
	}
	return nil
}

func validatePutContentType(ct string) (string, error) {
	clean := strings.TrimSpace(strings.ToLower(ct))
	if !allowedPutContentTypes[clean] {
		return "", fmt.Errorf("unsupported content type %q, allowed: image/jpeg, image/png", ct)
	}
	return clean, nil
}

// PresignedPutURL generates a temporary URL for direct upload: Phone -> storage.
// The caller must send the exact Content-Type signed here, or the
// signature will fail to validate.
func (s *R2Service) PresignedPutURL(ctx context.Context, key string, contentType string, expires time.Duration) (string, error) {
	if ctx == nil {
		return "", errors.New("ctx must not be nil")
	}
	if err := validateExpiry(expires); err != nil {
		return "", err
	}
	cleanKey, err := s.sanitizeKey(key)
	if err != nil {
		return "", err
	}
	cleanCT, err := validatePutContentType(contentType)
	if err != nil {
		return "", err
	}
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.BucketName),
		Key:         aws.String(cleanKey),
		ContentType: aws.String(cleanCT),
	}
	out, err := s.presigner.PresignPutObject(ctx, input, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("failed to presign PUT URL: %w", err)
	}
	return out.URL, nil
}

// PresignedGetURL generates a temporary URL for fetching: storage -> Agnes.
func (s *R2Service) PresignedGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if ctx == nil {
		return "", errors.New("ctx must not be nil")
	}
	if err := validateExpiry(expires); err != nil {
		return "", err
	}
	cleanKey, err := s.sanitizeKey(key)
	if err != nil {
		return "", err
	}
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.BucketName),
		Key:    aws.String(cleanKey),
	}
	out, err := s.presigner.PresignGetObject(ctx, input, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("failed to presign GET URL: %w", err)
	}
	return out.URL, nil
}

// DeleteObject removes an object from storage. Used for cleanup.
func (s *R2Service) DeleteObject(ctx context.Context, key string) error {
	if ctx == nil {
		return errors.New("ctx must not be nil")
	}
	cleanKey, err := s.sanitizeKey(key)
	if err != nil {
		return err
	}
	_, err = s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.BucketName),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// BucketName returns the configured bucket name. No secrets are exposed.
func (s *R2Service) BucketName() string {
	return s.cfg.BucketName
}
