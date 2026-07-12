package storage

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashScopeUploaderUploadReturnsOSSSchemeURL(t *testing.T) {
	audio := writeTempAudio(t, "hello-audio")

	var uploadHost string
	var gotAuth string
	var gotModel string
	gotFields := map[string]string{}
	var gotFileBody string

	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Errorf("parse upload content type: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("read multipart: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(part)
			if part.FormName() == "file" {
				gotFileBody = string(body)
				continue
			}
			gotFields[part.FormName()] = string(body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer uploadServer.Close()
	uploadHost = uploadServer.URL

	policyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotModel = r.URL.Query().Get("model")
		if r.URL.Query().Get("action") != "getPolicy" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"policy":                 "test-policy",
				"signature":              "test-signature",
				"upload_dir":             "dashscope-instant/acct/2026-07-12/uuid",
				"upload_host":            uploadHost,
				"oss_access_key_id":      "test-ak",
				"x_oss_object_acl":       "private",
				"x_oss_forbid_overwrite": "false",
				"max_file_size_mb":       1024,
			},
		})
	}))
	defer policyServer.Close()

	uploader, err := NewDashScopeUploader("sk-test-key", "fun-asr")
	if err != nil {
		t.Fatalf("new uploader: %v", err)
	}
	uploader.PolicyURL = policyServer.URL

	uploaded, err := uploader.Upload(context.Background(), audio)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	if gotAuth != "Bearer sk-test-key" {
		t.Errorf("policy Authorization = %q, want Bearer sk-test-key", gotAuth)
	}
	if gotModel != "fun-asr" {
		t.Errorf("policy model = %q, want fun-asr", gotModel)
	}
	if gotFileBody != "hello-audio" {
		t.Errorf("uploaded file body = %q, want hello-audio", gotFileBody)
	}
	if gotFields["OSSAccessKeyId"] != "test-ak" || gotFields["Signature"] != "test-signature" || gotFields["policy"] != "test-policy" {
		t.Errorf("upload form fields not forwarded from policy: %#v", gotFields)
	}

	wantPrefix := "oss://dashscope-instant/acct/2026-07-12/uuid/"
	if !strings.HasPrefix(uploaded.ReadURL, wantPrefix) {
		t.Errorf("ReadURL = %q, want prefix %q", uploaded.ReadURL, wantPrefix)
	}
	if uploaded.ObjectKey != strings.TrimPrefix(uploaded.ReadURL, "oss://") {
		t.Errorf("ObjectKey %q should match ReadURL %q without scheme", uploaded.ObjectKey, uploaded.ReadURL)
	}
	if gotFields["key"] != uploaded.ObjectKey {
		t.Errorf("upload key %q != returned ObjectKey %q", gotFields["key"], uploaded.ObjectKey)
	}
}

func TestDashScopeUploaderDeleteIsNoop(t *testing.T) {
	uploader, err := NewDashScopeUploader("sk-test-key", "")
	if err != nil {
		t.Fatalf("new uploader: %v", err)
	}
	if err := uploader.Delete(context.Background(), "any/key"); err != nil {
		t.Errorf("Delete should be a no-op, got %v", err)
	}
}

func TestNewDashScopeUploaderRequiresAPIKey(t *testing.T) {
	if _, err := NewDashScopeUploader("   ", "fun-asr"); err == nil {
		t.Error("expected error when api key is blank")
	}
}

func TestOSSConfigIsConfigured(t *testing.T) {
	full := OSSConfig{AccessKeyID: "ak", AccessKeySecret: "sk", Bucket: "b", Endpoint: "e"}
	if !full.IsConfigured() {
		t.Error("complete OSS config should report configured")
	}
	if (OSSConfig{}).IsConfigured() {
		t.Error("empty OSS config should report not configured")
	}
	partial := full
	partial.Bucket = ""
	if partial.IsConfigured() {
		t.Error("OSS config missing bucket should report not configured")
	}
}

func writeTempAudio(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip.wav")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp audio: %v", err)
	}
	return path
}
