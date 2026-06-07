package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	aliyunoss "github.com/aliyun/aliyun-oss-go-sdk/oss"
)

const (
	defaultReadURLTTL      = 2 * time.Hour
	defaultObjectKeyPrefix = "asr-temp/video2md/"
)

var invalidObjectPartRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type UploadedObject struct {
	ObjectKey string
	ReadURL   string
}

type OSSConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	Bucket          string
	Endpoint        string
	ObjectKeyPrefix string
	ReadURLTTL      time.Duration
}

type AliyunOSSUploader struct {
	bucket          *aliyunoss.Bucket
	objectKeyPrefix string
	readURLTTL      time.Duration
}

func OSSConfigFromEnv() OSSConfig {
	return OSSConfig{
		AccessKeyID:     firstEnv("OSS_ACCESS_KEY_ID", "ALICLOUD_ACCESS_KEY_ID"),
		AccessKeySecret: firstEnv("OSS_ACCESS_KEY_SECRET", "ALICLOUD_ACCESS_KEY_SECRET"),
		Bucket:          os.Getenv("OSS_BUCKET"),
		Endpoint:        os.Getenv("OSS_ENDPOINT"),
		ObjectKeyPrefix: firstEnv("OSS_OBJECT_KEY_PREFIX", "OSS_PREFIX"),
		ReadURLTTL:      durationEnv("OSS_READ_URL_TTL", defaultReadURLTTL),
	}
}

func NewAliyunOSSUploader(config OSSConfig) (*AliyunOSSUploader, error) {
	normalized, err := normalizeOSSConfig(config)
	if err != nil {
		return nil, err
	}
	client, err := aliyunoss.New(
		normalized.Endpoint,
		normalized.AccessKeyID,
		normalized.AccessKeySecret,
	)
	if err != nil {
		return nil, fmt.Errorf("create oss client: %w", err)
	}
	bucket, err := client.Bucket(normalized.Bucket)
	if err != nil {
		return nil, fmt.Errorf("open oss bucket: %w", err)
	}
	return &AliyunOSSUploader{
		bucket:          bucket,
		objectKeyPrefix: normalizeObjectKeyPrefix(normalized.ObjectKeyPrefix),
		readURLTTL:      normalized.ReadURLTTL,
	}, nil
}

func (u *AliyunOSSUploader) Upload(ctx context.Context, filePath string) (UploadedObject, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return UploadedObject{}, err
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return UploadedObject{}, err
	}
	if info.IsDir() || info.Size() <= 0 {
		return UploadedObject{}, fmt.Errorf("upload file must be a non-empty file: %s", filePath)
	}

	objectKey, err := buildObjectKey(u.objectKeyPrefix, filepath.Base(filePath))
	if err != nil {
		return UploadedObject{}, err
	}
	contentType := contentTypeForPath(filePath)
	if err := u.bucket.PutObjectFromFile(
		objectKey,
		filePath,
		aliyunoss.ContentType(contentType),
	); err != nil {
		return UploadedObject{}, fmt.Errorf("upload to oss: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return UploadedObject{}, err
	}
	readURL, err := u.bucket.SignURL(objectKey, aliyunoss.HTTPGet, int64(u.readURLTTL.Seconds()))
	if err != nil {
		return UploadedObject{}, fmt.Errorf("sign oss read url: %w", err)
	}
	return UploadedObject{ObjectKey: objectKey, ReadURL: readURL}, nil
}

func (u *AliyunOSSUploader) Delete(ctx context.Context, objectKey string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	key := normalizeObjectKey(objectKey)
	if key == "" {
		return nil
	}
	return u.bucket.DeleteObject(key)
}

func normalizeOSSConfig(config OSSConfig) (OSSConfig, error) {
	out := config
	out.AccessKeyID = strings.TrimSpace(out.AccessKeyID)
	out.AccessKeySecret = strings.TrimSpace(out.AccessKeySecret)
	out.Bucket = strings.TrimSpace(out.Bucket)
	out.Endpoint = normalizeEndpoint(out.Endpoint)
	out.ObjectKeyPrefix = normalizeObjectKeyPrefix(out.ObjectKeyPrefix)
	if out.ObjectKeyPrefix == "" {
		out.ObjectKeyPrefix = defaultObjectKeyPrefix
	}
	if out.ReadURLTTL <= 0 {
		out.ReadURLTTL = defaultReadURLTTL
	}
	if out.AccessKeyID == "" {
		return out, fmt.Errorf("OSS_ACCESS_KEY_ID or ALICLOUD_ACCESS_KEY_ID is required")
	}
	if out.AccessKeySecret == "" {
		return out, fmt.Errorf("OSS_ACCESS_KEY_SECRET or ALICLOUD_ACCESS_KEY_SECRET is required")
	}
	if out.Bucket == "" {
		return out, fmt.Errorf("OSS_BUCKET is required")
	}
	if out.Endpoint == "" {
		return out, fmt.Errorf("OSS_ENDPOINT is required")
	}
	return out, nil
}

func normalizeEndpoint(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}

func buildObjectKey(prefix string, filename string) (string, error) {
	randomPart := make([]byte, 5)
	if _, err := rand.Read(randomPart); err != nil {
		return "", fmt.Errorf("generate object key random part: %w", err)
	}
	today := time.Now().UTC().Format("2006/01/02")
	return fmt.Sprintf(
		"%s%s/%s_%d_%s",
		normalizeObjectKeyPrefix(prefix),
		today,
		hex.EncodeToString(randomPart),
		time.Now().UTC().Unix(),
		sanitizeObjectFileName(path.Base(filename)),
	), nil
}

func sanitizeObjectFileName(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "audio.wav"
	}
	extWithDot := filepath.Ext(value)
	ext := strings.TrimPrefix(strings.ToLower(extWithDot), ".")
	base := strings.TrimSuffix(value, extWithDot)
	base = strings.ReplaceAll(strings.TrimSpace(base), " ", "_")
	base = invalidObjectPartRegex.ReplaceAllString(base, "_")
	base = strings.Trim(base, "._-")
	if base == "" {
		base = "audio"
	}
	ext = invalidObjectPartRegex.ReplaceAllString(ext, "")
	ext = strings.Trim(ext, "._-")
	if ext == "" {
		ext = "wav"
	}
	return base + "." + ext
}

func normalizeObjectKeyPrefix(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	parts := strings.Split(value, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		part = invalidObjectPartRegex.ReplaceAllString(part, "_")
		part = strings.Trim(part, "._-")
		if part == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	if len(cleaned) == 0 {
		return ""
	}
	return strings.Join(cleaned, "/") + "/"
}

func normalizeObjectKey(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.ReplaceAll(value, "\\", "/")
	parts := strings.Split(value, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		part = invalidObjectPartRegex.ReplaceAllString(part, "_")
		part = strings.Trim(part, "._-")
		if part == "" {
			continue
		}
		cleaned = append(cleaned, part)
	}
	return strings.Join(cleaned, "/")
}

func contentTypeForPath(filePath string) string {
	if contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath))); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
