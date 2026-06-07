package storage

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeOSSConfig_UsesDefaultsAndNormalizesEndpoint(t *testing.T) {
	got, err := normalizeOSSConfig(OSSConfig{
		AccessKeyID:     "ak",
		AccessKeySecret: "secret",
		Bucket:          "bucket",
		Endpoint:        "oss-cn-shanghai.aliyuncs.com",
	})
	if err != nil {
		t.Fatalf("normalizeOSSConfig returned error: %v", err)
	}
	if got.Endpoint != "https://oss-cn-shanghai.aliyuncs.com" {
		t.Fatalf("endpoint = %q", got.Endpoint)
	}
	if got.ObjectKeyPrefix != defaultObjectKeyPrefix {
		t.Fatalf("prefix = %q", got.ObjectKeyPrefix)
	}
	if got.ReadURLTTL != defaultReadURLTTL {
		t.Fatalf("ttl = %s", got.ReadURLTTL)
	}
}

func TestOSSConfigFromEnv_UsesOSSAndAliCloudFallbacks(t *testing.T) {
	t.Setenv("OSS_ACCESS_KEY_ID", "")
	t.Setenv("OSS_ACCESS_KEY_SECRET", "")
	t.Setenv("ALICLOUD_ACCESS_KEY_ID", "ak-fallback")
	t.Setenv("ALICLOUD_ACCESS_KEY_SECRET", "secret-fallback")
	t.Setenv("OSS_BUCKET", "bucket")
	t.Setenv("OSS_ENDPOINT", "endpoint")
	t.Setenv("OSS_READ_URL_TTL", "30m")

	got := OSSConfigFromEnv()
	if got.AccessKeyID != "ak-fallback" {
		t.Fatalf("access key id = %q", got.AccessKeyID)
	}
	if got.AccessKeySecret != "secret-fallback" {
		t.Fatalf("access key secret fallback not used")
	}
	if got.ReadURLTTL != 30*time.Minute {
		t.Fatalf("ttl = %s", got.ReadURLTTL)
	}
}

func TestBuildObjectKey_SanitizesFilenameAndPrefix(t *testing.T) {
	got, err := buildObjectKey("../asr temp//", "会议 记录#1.wav")
	if err != nil {
		t.Fatalf("buildObjectKey returned error: %v", err)
	}
	if !strings.HasPrefix(got, "asr_temp/") {
		t.Fatalf("object key prefix not normalized: %q", got)
	}
	if strings.Contains(got, "#") || strings.Contains(got, " ") {
		t.Fatalf("object key was not sanitized: %q", got)
	}
	if !strings.Contains(got, "1.wav") {
		t.Fatalf("object key missing sanitized extension: %q", got)
	}
}
