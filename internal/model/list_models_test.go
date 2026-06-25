package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// openaiModelsPage 模拟 OpenAI 官方 /v1/models 的单页响应。
func openaiModelsPage(ids []string, next string) string {
	data := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		data = append(data, map[string]any{"id": id, "object": "model"})
	}
	body, _ := json.Marshal(map[string]any{"object": "list", "data": data})
	if next == "" {
		return string(body)
	}
	// 翻页用 next_page_url 字段；OpenAI SDK 的 ListAutoPaging 在 raw HTTP 实现下
	// 会把 next_page_url 当 raw body 读为下一次 list，再走我们的 decode 路径。
	full, _ := json.Marshal(map[string]any{"object": "list", "data": data, "has_more": true, "next_page_url": next})
	return string(full)
}

// anthropicModelsBody 模拟 Anthropic 官方 /v1/models 的响应。
func anthropicModelsBody(ids []string) string {
	data := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		data = append(data, map[string]string{"id": id, "type": "model"})
	}
	body, _ := json.Marshal(map[string]any{"data": data, "first_id": "", "has_more": false, "last_id": ""})
	return string(body)
}

func TestOpenAIListModelsParsesSinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(openaiModelsPage([]string{"gpt-4o", "gpt-4o-mini", "o1-preview"}, "")))
	}))
	defer srv.Close()

	got, err := openaiListModels(context.Background(), "test-key", srv.URL+"/v1")
	if err != nil {
		t.Fatalf("openaiListModels() error = %v", err)
	}
	want := []string{"gpt-4o", "gpt-4o-mini", "o1-preview"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOpenAIListModelsRejectsEmptyKey(t *testing.T) {
	if _, err := openaiListModels(context.Background(), "", "https://example.com/v1"); err == nil {
		t.Fatal("expected error for empty api_key, got nil")
	}
	if _, err := openaiListModels(context.Background(), "k", ""); err == nil {
		t.Fatal("expected error for empty base_url, got nil")
	}
}

func TestOpenAIListModelsReturnsErrorOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()
	_, err := openaiListModels(context.Background(), "k", srv.URL+"/v1")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error %q should mention status 401", err)
	}
}

func TestAnthropicListModelsParsesResponse(t *testing.T) {
	var sawAuth, sawVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		sawAuth = r.Header.Get("x-api-key")
		sawVersion = r.Header.Get("anthropic-version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(anthropicModelsBody([]string{"claude-opus-4-20250514", "claude-sonnet-4-20250514"})))
	}))
	defer srv.Close()

	got, err := anthropicListModels(context.Background(), "test-key", srv.URL)
	if err != nil {
		t.Fatalf("anthropicListModels() error = %v", err)
	}
	if sawAuth != "test-key" {
		t.Errorf("x-api-key = %q, want test-key", sawAuth)
	}
	if sawVersion == "" {
		t.Error("anthropic-version header not set")
	}
	want := []string{"claude-opus-4-20250514", "claude-sonnet-4-20250514"}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAnthropicListModelsRejectsEmptyInputs(t *testing.T) {
	if _, err := anthropicListModels(context.Background(), "", "https://example.com"); err == nil {
		t.Fatal("expected error for empty api_key")
	}
	if _, err := anthropicListModels(context.Background(), "k", ""); err == nil {
		t.Fatal("expected error for empty base_url")
	}
}

func TestAnthropicListModelsReturnsErrorOn500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	_, err := anthropicListModels(context.Background(), "k", srv.URL)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error %q should mention status 500", err)
	}
}

// TestNewProviderForListingRoutesProviders 验证 listing-only helper 按 provider 类型走对应 SDK。
// 关键点：返回 *AnthropicProvider 走 anthropic 协议（自动在 baseURL 后拼 /v1/models），
// 两个 OpenAI 实现直接走 baseURL + /models。
func TestNewProviderForListingRoutesProviders(t *testing.T) {
	cases := []struct {
		provider string
		// baseURLSuffix 是 NewProviderForListing 收到的 baseURL 后缀。
		// anthropic 自身拼 /v1/models，所以 baseURL 不带 /v1；
		// openai/openai-compatible 直接拼 /models，baseURL 需带 /v1。
		baseURLSuffix string
		wantPath      string
		wantKey       string
	}{
		{provider: "anthropic", baseURLSuffix: "", wantPath: "/v1/models", wantKey: "x-api-key"},
		{provider: "openai", baseURLSuffix: "/v1", wantPath: "/v1/models", wantKey: "Authorization"},
		{provider: "openai-compatible", baseURLSuffix: "/v1", wantPath: "/v1/models", wantKey: "Authorization"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.provider, func(t *testing.T) {
			var sawPath, sawKey, sawValue string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sawPath = r.URL.Path
				sawKey = r.Header.Get(tc.wantKey)
				w.Header().Set("Content-Type", "application/json")
				if tc.provider == "anthropic" {
					_, _ = w.Write([]byte(anthropicModelsBody([]string{"claude-test"})))
				} else {
					_, _ = w.Write([]byte(openaiModelsPage([]string{"gpt-test"}, "")))
				}
			}))
			defer srv.Close()
			prov, err := NewProviderForListing(tc.provider, srv.URL+tc.baseURLSuffix, "k")
			if err != nil {
				t.Fatalf("NewProviderForListing(%q) error = %v", tc.provider, err)
			}
			ids, err := prov.ListModels(context.Background())
			if err != nil {
				t.Fatalf("ListModels() error = %v", err)
			}
			sawValue = strings.Join(ids, ",")
			if sawPath != tc.wantPath {
				t.Errorf("path = %q, want %q", sawPath, tc.wantPath)
			}
			if sawKey == "" {
				t.Errorf("expected header %q to be set", tc.wantKey)
			}
			if sawValue == "" {
				t.Errorf("expected at least one model id, got %q", sawValue)
			}
		})
	}
}

func TestNewProviderForListingRejectsEmpty(t *testing.T) {
	if _, err := NewProviderForListing("openai", "https://example.com", ""); err == nil {
		t.Error("expected error for empty api_key")
	}
	if _, err := NewProviderForListing("openai", "", "k"); err == nil {
		t.Error("expected error for empty base_url")
	}
}

// TestOpenAIListModelsHandlesBaseURLTrailingSlash 验证 baseURL 带或不带尾斜杠都能正确拼路径。
func TestOpenAIListModelsHandlesBaseURLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q, want /v1/models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(openaiModelsPage([]string{"x"}, "")))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	base := u.String() + "/v1/"
	if _, err := openaiListModels(context.Background(), "k", base); err != nil {
		t.Fatalf("openaiListModels() error = %v", err)
	}
	// 同时验证 truncateForError 在错误路径上能用
	if got := truncateForError([]byte("hello")); got != "hello" {
		t.Errorf("truncateForError(short) = %q", got)
	}
	big := make([]byte, 500)
	for i := range big {
		big[i] = 'a'
	}
	if got := truncateForError(big); !strings.HasSuffix(got, "…") {
		t.Errorf("truncateForError(long) should end with ellipsis, got %q", got)
	}
	// 防止 lint 警告：保留未使用的 fmt 包
	_ = fmt.Sprintf
}
