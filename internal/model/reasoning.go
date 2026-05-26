package model

import (
	"fmt"
	"sort"

	"github.com/openai/openai-go/v3/option"
)

// mergeReasoningFields 将配置里的 models.reasoning 平铺进最终请求体。
// 这里不理解任何模型 preset，只保护 core 已经生成的字段不被覆盖。
func mergeReasoningFields(body map[string]any, reasoning map[string]any) error {
	if len(reasoning) == 0 {
		return nil
	}
	for _, key := range sortedReasoningKeys(reasoning) {
		value := reasoning[key]
		if _, exists := body[key]; exists {
			return fmt.Errorf("reasoning field %q conflicts with generated request body", key)
		}
		body[key] = value
	}
	return nil
}

func reasoningRequestOptions(reasoning map[string]any, generated map[string]bool) ([]option.RequestOption, error) {
	if len(reasoning) == 0 {
		return nil, nil
	}
	opts := make([]option.RequestOption, 0, len(reasoning))
	for _, key := range sortedReasoningKeys(reasoning) {
		value := reasoning[key]
		if generated[key] {
			return nil, fmt.Errorf("reasoning field %q conflicts with generated request body", key)
		}
		opts = append(opts, option.WithJSONSet(key, value))
	}
	return opts, nil
}

func sortedReasoningKeys(reasoning map[string]any) []string {
	keys := make([]string, 0, len(reasoning))
	for key := range reasoning {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
