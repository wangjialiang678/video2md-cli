package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultDashScopeUploadPolicyURL = "https://dashscope.aliyuncs.com/api/v1/uploads"
	defaultDashScopeModel           = "fun-asr"
	defaultDashScopeHTTPTimeout     = 10 * time.Minute
)

// DashScopeUploader 把音频上传到 DashScope 自带的临时文件空间，换取 oss:// URL。
// 相比 AliyunOSSUploader，它不需要用户自建 bucket，只用 DASHSCOPE_API_KEY 一个凭证。
// 临时文件由 DashScope 侧在 48 小时后自动过期，因此 Delete 是空操作。
type DashScopeUploader struct {
	APIKey     string
	Model      string
	PolicyURL  string
	HTTPClient *http.Client
}

type dashScopeUploadPolicy struct {
	Policy              string `json:"policy"`
	Signature           string `json:"signature"`
	UploadDir           string `json:"upload_dir"`
	UploadHost          string `json:"upload_host"`
	OSSAccessKeyID      string `json:"oss_access_key_id"`
	XOSSObjectACL       string `json:"x_oss_object_acl"`
	XOSSForbidOverwrite string `json:"x_oss_forbid_overwrite"`
	MaxFileSizeMB       int64  `json:"max_file_size_mb"`
}

func NewDashScopeUploader(apiKey string, model string) (*DashScopeUploader, error) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return nil, fmt.Errorf("DASHSCOPE_API_KEY is required")
	}
	name := strings.TrimSpace(model)
	if name == "" {
		name = defaultDashScopeModel
	}
	return &DashScopeUploader{
		APIKey:     key,
		Model:      name,
		PolicyURL:  defaultDashScopeUploadPolicyURL,
		HTTPClient: &http.Client{Timeout: defaultDashScopeHTTPTimeout},
	}, nil
}

func (u *DashScopeUploader) Upload(ctx context.Context, filePath string) (UploadedObject, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return UploadedObject{}, err
	}
	if info.IsDir() || info.Size() <= 0 {
		return UploadedObject{}, fmt.Errorf("upload file must be a non-empty file: %s", filePath)
	}

	policy, err := u.getPolicy(ctx)
	if err != nil {
		return UploadedObject{}, err
	}
	if policy.MaxFileSizeMB > 0 && info.Size() > policy.MaxFileSizeMB*1024*1024 {
		return UploadedObject{}, fmt.Errorf(
			"audio chunk %s is %d MB, exceeds DashScope temporary storage limit of %d MB",
			filepath.Base(filePath), info.Size()/(1024*1024), policy.MaxFileSizeMB,
		)
	}

	objectKey := strings.TrimSuffix(policy.UploadDir, "/") + "/" + sanitizeObjectFileName(filepath.Base(filePath))
	if err := u.postFile(ctx, policy, objectKey, filePath); err != nil {
		return UploadedObject{}, err
	}
	return UploadedObject{ObjectKey: objectKey, ReadURL: "oss://" + objectKey}, nil
}

// Delete 是空操作：DashScope 临时空间的对象 48 小时后自动过期，客户端无权也无需删除。
func (u *DashScopeUploader) Delete(_ context.Context, _ string) error {
	return nil
}

func (u *DashScopeUploader) getPolicy(ctx context.Context) (dashScopeUploadPolicy, error) {
	url := fmt.Sprintf("%s?action=getPolicy&model=%s", u.policyURL(), u.Model)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return dashScopeUploadPolicy{}, err
	}
	request.Header.Set("Authorization", "Bearer "+u.APIKey)

	response, err := u.httpClient().Do(request)
	if err != nil {
		return dashScopeUploadPolicy{}, fmt.Errorf("request dashscope upload policy: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return dashScopeUploadPolicy{}, err
	}
	if response.StatusCode != http.StatusOK {
		return dashScopeUploadPolicy{}, fmt.Errorf(
			"dashscope upload policy failed with status %d: %s",
			response.StatusCode, truncate(string(body), 300),
		)
	}

	var payload struct {
		Data dashScopeUploadPolicy `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return dashScopeUploadPolicy{}, fmt.Errorf("decode dashscope upload policy: %w", err)
	}
	if strings.TrimSpace(payload.Data.UploadHost) == "" || strings.TrimSpace(payload.Data.UploadDir) == "" {
		return dashScopeUploadPolicy{}, fmt.Errorf("dashscope upload policy is incomplete: %s", truncate(string(body), 300))
	}
	return payload.Data, nil
}

func (u *DashScopeUploader) postFile(ctx context.Context, policy dashScopeUploadPolicy, objectKey string, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	fields := [][2]string{
		{"OSSAccessKeyId", policy.OSSAccessKeyID},
		{"Signature", policy.Signature},
		{"policy", policy.Policy},
		{"key", objectKey},
		{"x-oss-object-acl", policy.XOSSObjectACL},
		{"x-oss-forbid-overwrite", policy.XOSSForbidOverwrite},
		{"success_action_status", "200"},
	}
	for _, field := range fields {
		if err := writer.WriteField(field[0], field[1]); err != nil {
			return err
		}
	}
	part, err := writer.CreateFormFile("file", filepath.Base(objectKey))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, policy.UploadHost, &buffer)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := u.httpClient().Do(request)
	if err != nil {
		return fmt.Errorf("upload to dashscope temporary storage: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf(
			"upload to dashscope temporary storage failed with status %d: %s",
			response.StatusCode, truncate(string(body), 300),
		)
	}
	return nil
}

func (u *DashScopeUploader) policyURL() string {
	if url := strings.TrimSpace(u.PolicyURL); url != "" {
		return url
	}
	return defaultDashScopeUploadPolicyURL
}

func (u *DashScopeUploader) httpClient() *http.Client {
	if u.HTTPClient != nil {
		return u.HTTPClient
	}
	return &http.Client{Timeout: defaultDashScopeHTTPTimeout}
}

func truncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
