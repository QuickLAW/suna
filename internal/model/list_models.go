package model

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// ListModelsHTTPTimeout 拉取模型列表的请求超时；标准协议的 /v1/models 响应通常很小。
const ListModelsHTTPTimeout = 15 * time.Second

// maxListModelsResponseSize 限制 /v1/models 响应体大小，防止恶意端点返回巨大响应导致 OOM。
// 正常模型列表响应通常不超过 100KB。
const maxListModelsResponseSize = 2 * 1024 * 1024 // 2MB

// listModelsHTTPClient 复用 compatibleHeaderRoundTripper 清理 Stainless 追踪头，
// 与 Suna 模型调用保持一致。
func listModelsHTTPClient() *http.Client {
	return compatibleHTTPClient(&http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}})
}

// openaiListModels 调 OpenAI 标准协议的 /v1/models（GET {baseURL}/models）。
// 适用于 OpenAI 官方以及任何兼容 OpenAI /v1/models 协议的供应商。
// 自动分页：使用 ListAutoPaging 翻页并只收集 ID。
func openaiListModels(ctx context.Context, apiKey, baseURL string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required to list models")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("base_url is required to list models")
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey), option.WithHTTPClient(listModelsHTTPClient()), option.WithMaxRetries(0), option.WithBaseURL(baseURL)}
	client := openai.NewClient(opts...)
	pager := client.Models.ListAutoPaging(ctx)
	ids := make([]string, 0, 32)
	for pager.Next() {
		m := pager.Current()
		if m.ID == "" {
			continue
		}
		ids = append(ids, m.ID)
	}
	if err := pager.Err(); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	return ids, nil
}

// anthropicListModels 调 Anthropic 标准 /v1/models。
// Anthropic 官方 SDK 未提供 models 列表接口，这里直接走 HTTP；请求体按 SDK 的
// normalizeCompatibleHeaders 清理 Stainless 追踪头，并设置官方必需的 anthropic-version。
func anthropicListModels(ctx context.Context, apiKey, baseURL string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("api_key is required to list models")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("base_url is required to list models")
	}
	trimmed := strings.TrimRight(baseURL, "/")
	url := trimmed + "/v1/models"
	httpClient := listModelsHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build models request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	normalizeCompatibleHeaders(req)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxListModelsResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read models response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("list models: status %d: %s", resp.StatusCode, truncateForError(body))
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse models response: %w", err)
	}
	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID == "" {
			continue
		}
		ids = append(ids, item.ID)
	}
	return ids, nil
}

func truncateForError(body []byte) string {
	const max = 240
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "…"
}
