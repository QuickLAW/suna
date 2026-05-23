package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alanchenchen/suna/internal/memory"
	"github.com/alanchenchen/suna/internal/model"
)

const contextSafetyThreshold = 0.8

func (r *Runner) Compact(ctx context.Context, working *memory.WorkingMemory) (before, after, turnsCompressed, truncated int, err error) {
	if r.Compressor == nil || working == nil {
		return 0, 0, 0, 0, fmt.Errorf("compressor not initialized")
	}
	msgs := working.Messages()
	before = working.EstimatedTokens()
	if len(msgs) <= memory.KeepRecentTurns {
		return before, before, 0, 0, nil
	}
	compressed, summary, compErr := r.Compressor.CompressHistory(ctx, msgs)
	if compErr != nil {
		return 0, 0, 0, 0, compErr
	}
	if summary == "" && len(compressed) == len(msgs) {
		return before, before, 0, 0, nil
	}
	turnsCompressed = len(msgs) - len(compressed)
	if turnsCompressed < 0 {
		turnsCompressed = 0
	}
	working.SetMessages(compressed)
	after = working.EstimatedTokens()
	for _, m := range msgs {
		if m.Role == model.RoleTool && len(m.Text()) > 50*1024 {
			truncated++
		}
	}
	return before, after, turnsCompressed, truncated, nil
}

func (r *Runner) compactForRequest(ctx context.Context, working *memory.WorkingMemory, req *model.CompletionRequest, contextWindow int) {
	if r.Compressor == nil || working == nil {
		return
	}
	if !shouldCompactRequest(req, contextWindow) {
		return
	}
	keep := chooseRecentKeepForRequest(req, working.Messages(), contextWindow)
	compressed, _, err := r.Compressor.CompressHistoryKeeping(ctx, working.Messages(), keep)
	if err == nil {
		working.SetMessages(compressed)
	}
}

func shouldCompactRequest(req *model.CompletionRequest, contextWindow int) bool {
	if req == nil || contextWindow <= 0 {
		return false
	}
	return estimateRequestTokens(req) > int(float64(contextWindow)*contextSafetyThreshold)
}

func estimateRequestTokens(req *model.CompletionRequest) int {
	if req == nil {
		return 0
	}
	total := model.EstimateTokens(req.System)
	total += model.EstimateMessagesTokens(req.Messages)
	total += req.MaxTokens
	if len(req.Tools) > 0 {
		if data, err := json.Marshal(req.Tools); err == nil {
			total += model.EstimateTokens(string(data))
		}
	}
	return total
}

func fixedRequestTokens(req *model.CompletionRequest) int {
	if req == nil {
		return 0
	}
	fixed := model.EstimateTokens(req.System) + req.MaxTokens
	if len(req.Tools) > 0 {
		if data, err := json.Marshal(req.Tools); err == nil {
			fixed += model.EstimateTokens(string(data))
		}
	}
	return fixed
}

func chooseRecentKeepForRequest(req *model.CompletionRequest, working []model.Message, contextWindow int) int {
	if len(working) <= 1 {
		return len(working)
	}
	budget := int(float64(contextWindow)*contextSafetyThreshold) - fixedRequestTokens(req) - nonWorkingMessageTokens(req, working)
	if budget <= 0 {
		return 1
	}
	keep := 0
	tokens := 0
	for i := len(working) - 1; i >= 0 && keep < memory.KeepRecentTurns; i-- {
		msgTokens := model.EstimateMessagesTokens([]model.Message{working[i]})
		if keep > 0 && tokens+msgTokens > budget {
			break
		}
		tokens += msgTokens
		keep++
		if tokens > budget {
			break
		}
	}
	if keep < 1 {
		keep = 1
	}
	if keep >= len(working) {
		keep = len(working) - 1
	}
	return keep
}

func nonWorkingMessageTokens(req *model.CompletionRequest, working []model.Message) int {
	if req == nil || len(req.Messages) <= len(working) {
		return 0
	}
	return model.EstimateMessagesTokens(req.Messages[:len(req.Messages)-len(working)])
}
