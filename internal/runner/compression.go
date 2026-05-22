package runner

import (
	"context"
	"fmt"

	"github.com/alanchenchen/suna/internal/memory"
	"github.com/alanchenchen/suna/internal/model"
)

func (r *Runner) Compact(ctx context.Context, working *memory.WorkingMemory) (before, after, turnsCompressed, truncated int, err error) {
	if r.Compressor == nil || working == nil {
		return 0, 0, 0, 0, fmt.Errorf("compressor not initialized")
	}
	msgs := working.Messages()
	if len(msgs) <= 10 {
		return 0, 0, 0, 0, fmt.Errorf("too few messages to compress (%d)", len(msgs))
	}
	before = working.EstimatedTokens()
	compressed, summary, compErr := r.Compressor.CompressHistory(ctx, msgs)
	if compErr != nil {
		return 0, 0, 0, 0, compErr
	}
	if summary == "" {
		return 0, 0, 0, 0, fmt.Errorf("compression produced no summary")
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

func (r *Runner) autoCompact(ctx context.Context, working *memory.WorkingMemory, contextWindow int) {
	if r.Compressor == nil || working == nil {
		return
	}
	msgs := working.Messages()
	shouldCompress := r.Compressor.ShouldCompress(msgs, contextWindow) ||
		(len(msgs) > memory.AutoCompactMinTurns && r.Compressor.EstimateTokens(msgs) > contextWindow/2)
	if !shouldCompress {
		return
	}
	compressed, _, err := r.Compressor.CompressHistory(ctx, msgs)
	if err == nil {
		working.SetMessages(compressed)
	}
}
