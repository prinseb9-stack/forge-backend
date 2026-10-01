package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func testR2Config() R2Config {
	return R2Config{
		AccountID:       "unit-test-account-do-not-use",
		AccessKeyID:     "unit-test-access-key-do-not-use",
		SecretAccessKey: "unit-test-secret-key-do-not-use",
		BucketName:      "unit-test-bucket-do-not-use",
	}
}

func TestLoadR2ConfigFromEnv_Missing(t *testing.T) {
	t.Setenv(EnvR2AccountID, "")
	t.Setenv(EnvR2AccessKeyID, "")
	t.Setenv(EnvR2SecretAccessKey, "")
	t.Setenv(EnvR2BucketName, "")

	_, err := LoadR2ConfigFromEnv()
	if err == nil {
		t.Fatal("expected error when env vars are missing")
	}
}

func TestNewR2Service_Validation(t *testing.T) {
	ctx := context.Background()
	base := testR2Config()

	cases := []struct {
		name   string
		mutate func(*R2Config)
	}{
		{"missing AccountID", func(c *R2Config) { c.AccountID = "" }},
		{"missing AccessKeyID", func(c *R2Config) { c.AccessKeyID = "" }},
		{"missing SecretAccessKey", func(c *R2Config) { c.SecretAccessKey = "" }},
		{"missing BucketName", func(c *R2Config) { c.BucketName = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)
			if _, err := NewR2Service(ctx, cfg); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestNewR2Service_Success(t *testing.T) {
	ctx := context.Background()
	svc, err := NewR2Service(ctx, testR2Config())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if svc.BucketName() != "unit-test-bucket-do-not-use" {
		t.Fatalf("unexpected bucket name: %s", svc.BucketName())
	}
}

func TestPresignedPutURL_Success(t *testing.T) {
	ctx := context.Background()
	svc, err := NewR2Service(ctx, testR2Config())
	if err != nil {
		t.Fatalf("NewR2Service failed: %v", err)
	}

	url, err := svc.PresignedPutURL(ctx, "uploads/test-image.jpg", "image/jpeg", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignedPutURL failed: %v", err)
	}
	if !strings.Contains(url, "unit-test-bucket-do-not-use") {
		t.Fatalf("PUT URL should contain bucket, got: %s", url)
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Fatalf("PUT URL should be signed, got: %s", url)
	}
}

func TestPresignedPutURL_RejectsDisallowedContentType(t *testing.T) {
	ctx := context.Background()
	svc, _ := NewR2Service(ctx, testR2Config())

	bad := []string{
		"application/x-msdownload",
		"text/html",
		"application/octet-stream",
		"",
	}
	for _, ct := range bad {
		if _, err := svc.PresignedPutURL(ctx, "uploads/a.jpg", ct, 15*time.Minute); err == nil {
			t.Fatalf("expected error for content type %q", ct)
		}
	}
}

func TestPresignedPutURL_NormalizesContentType(t *testing.T) {
	ctx := context.Background()
	svc, _ := NewR2Service(ctx, testR2Config())

	// Mixed case and whitespace should normalize to "image/jpeg"
	if _, err := svc.PresignedPutURL(ctx, "uploads/a.jpg", "  IMAGE/JPEG  ", 15*time.Minute); err != nil {
		t.Fatalf("expected normalized content type to succeed, got: %v", err)
	}
}

func TestPresignedGetURL_Success(t *testing.T) {
	ctx := context.Background()
	svc, _ := NewR2Service(ctx, testR2Config())

	url, err := svc.PresignedGetURL(ctx, "uploads/test-image.jpg", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignedGetURL failed: %v", err)
	}
	if !strings.Contains(url, "unit-test-bucket-do-not-use") {
		t.Fatalf("GET URL should contain bucket, got: %s", url)
	}
}

func TestPresignedURL_InvalidKey(t *testing.T) {
	ctx := context.Background()
	svc, _ := NewR2Service(ctx, testR2Config())

	if _, err := svc.PresignedPutURL(ctx, "   ", "image/jpeg", time.Minute); err == nil {
		t.Fatal("expected error for empty PUT key")
	}
	if _, err := svc.PresignedGetURL(ctx, "", time.Minute); err == nil {
		t.Fatal("expected error for empty GET key")
	}
	if _, err := svc.PresignedPutURL(ctx, "a\\b.jpg", "image/jpeg", time.Minute); err == nil {
		t.Fatal("expected error for backslash in key")
	}
	if _, err := svc.PresignedPutURL(ctx, "../escape.jpg", "image/jpeg", time.Minute); err == nil {
		t.Fatal("expected error for parent-directory traversal")
	}
}

func TestPresignedURL_InvalidExpiry(t *testing.T) {
	ctx := context.Background()
	svc, _ := NewR2Service(ctx, testR2Config())

	if _, err := svc.PresignedPutURL(ctx, "a.jpg", "image/jpeg", 0); err == nil {
		t.Fatal("expected error for zero expiry")
	}
	if _, err := svc.PresignedGetURL(ctx, "a.jpg", 8*24*time.Hour); err == nil {
		t.Fatal("expected error for expiry > 7 days")
	}
}

func TestDeleteObject_InvalidKey(t *testing.T) {
	ctx := context.Background()
	svc, _ := NewR2Service(ctx, testR2Config())

	if err := svc.DeleteObject(ctx, ""); err == nil {
		t.Fatal("expected error for empty delete key")
	}
}

// Live integration test — skipped unless FORGE_R2_LIVE_TEST=1.
// Uses real env vars. Never prints secret values.
func TestLiveR2Presign_Integration(t *testing.T) {
	if os.Getenv("FORGE_R2_LIVE_TEST") != "1" {
		t.Skip("skipping live R2 test; set FORGE_R2_LIVE_TEST=1 to run")
	}
	ctx := context.Background()

	cfg, err := LoadR2ConfigFromEnv()
	if err != nil {
		t.Fatalf("live test requires R2 env vars: %v", err)
	}
	svc, err := NewR2Service(ctx, cfg)
	if err != nil {
		t.Fatalf("NewR2Service failed: %v", err)
	}

	url, err := svc.PresignedPutURL(ctx, "integration-test/placeholder.jpg", "image/jpeg", 5*time.Minute)
	if err != nil {
		t.Fatalf("live PresignedPutURL failed: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Fatalf("expected signed URL, got: %s", url)
	}
}
