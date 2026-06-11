package memory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alanchenchen/suna/internal/logging"
	"github.com/google/uuid"
)

const maxQueueAttempts = 3

type QueueItem struct {
	ID            string
	UserID        string
	Kind          string
	Content       string
	Tags          []string
	Source        string
	Confidence    float64
	Evidence      string
	Significance  Significance
	CreatedAt     time.Time
	NextAttemptAt time.Time
	Attempts      int
}

type ExtractQueue struct {
	ch chan struct{}
	db *sql.DB
}

const extractQueueSize = 1

func NewExtractQueue(db *sql.DB) *ExtractQueue {
	return &ExtractQueue{ch: make(chan struct{}, extractQueueSize), db: db}
}

func (q *ExtractQueue) Push(ctx context.Context, userID string, candidate Candidate) bool {
	if q == nil || q.db == nil {
		return false
	}
	if userID == "" {
		userID = DefaultUserID
	}
	candidate.UserID = userID
	candidate, ok := normalizeCandidate(candidate)
	if !ok {
		return false
	}
	// memory_queue 只保存结构化用户画像候选，不保存原始对话，避免 assistant 总结或任务日志进入长期记忆。
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO memory_queue (id, user_id, kind, content, tags, source, confidence, evidence, significance, created_at, next_attempt_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uuid.New().String(), userID, candidate.Kind, candidate.Content, marshalStringSlice(candidate.Tags), candidate.Source, candidate.Confidence, candidate.Evidence, string(candidate.Significance), time.Now(), time.Now())
	if err != nil {
		logging.Error("memory", "queue_insert_failed", err, logging.Event{"queue_kind": candidate.Kind, "significance": string(candidate.Significance)})
		return false
	}
	q.Signal()
	return true
}

func (q *ExtractQueue) Signal() {
	select {
	case q.ch <- struct{}{}:
	default:
	}
}

func (q *ExtractQueue) Ch() <-chan struct{} { return q.ch }

func (q *ExtractQueue) Close() { close(q.ch) }

func (q *ExtractQueue) RecoverUnextracted(ctx context.Context) (int, error) {
	if q == nil || q.db == nil {
		return 0, nil
	}
	var count int
	if err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_queue WHERE processed_at IS NULL`).Scan(&count); err != nil {
		return 0, fmt.Errorf("recover memory queue: %w", err)
	}
	if count > 0 {
		q.Signal()
		logging.Info("memory", "queue_recovered", logging.Event{"pending_queue_events": count})
	}
	return count, nil
}

func LoadPendingQueue(ctx context.Context, db *sql.DB, userID string, limit int) ([]QueueItem, error) {
	if userID == "" {
		userID = DefaultUserID
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, kind, content, tags, source, confidence, evidence, significance, created_at, next_attempt_at, attempts
		FROM memory_queue
		WHERE user_id = ?
		  AND processed_at IS NULL
		  AND attempts < ?
		  AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
		ORDER BY created_at ASC
		LIMIT ?`, userID, maxQueueAttempts, time.Now(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []QueueItem
	for rows.Next() {
		var item QueueItem
		var tags, sig, created, nextAttempt sql.NullString
		if err := rows.Scan(&item.ID, &item.UserID, &item.Kind, &item.Content, &tags, &item.Source, &item.Confidence, &item.Evidence, &sig, &created, &nextAttempt, &item.Attempts); err != nil {
			continue
		}
		item.Kind = normalizeKind(item.Kind)
		item.Tags = normalizeTags(unmarshalStringSlice(tags.String))
		item.Source = normalizeSource(item.Source)
		item.Confidence = clampConfidence(item.Confidence)
		item.Significance = Significance(sig.String)
		item.CreatedAt = parseDBTime(created.String)
		item.NextAttemptAt = parseDBTime(nextAttempt.String)
		out = append(out, item)
	}
	return out, rows.Err()
}

func DeleteQueueItems(ctx context.Context, db *sql.DB, ids []string) error {
	for _, id := range ids {
		if _, err := db.ExecContext(ctx, `DELETE FROM memory_queue WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return nil
}

func RetryQueueItems(ctx context.Context, db *sql.DB, ids []string, cause error) error {
	if len(ids) == 0 {
		return nil
	}
	errText := ""
	if cause != nil {
		errText = truncateRunes(cause.Error(), 500)
	}
	for _, id := range ids {
		var attempts int
		_ = db.QueryRowContext(ctx, `SELECT attempts FROM memory_queue WHERE id = ?`, id).Scan(&attempts)
		nextAttempts := attempts + 1
		if nextAttempts >= maxQueueAttempts {
			if _, err := db.ExecContext(ctx, `DELETE FROM memory_queue WHERE id = ?`, id); err != nil {
				return err
			}
			logging.Error("memory", "queue_drop_after_retries", cause, logging.Event{"attempts": nextAttempts, "queue_id": id})
			continue
		}
		next := time.Now().Add(queueBackoff(nextAttempts))
		if _, err := db.ExecContext(ctx, `UPDATE memory_queue SET attempts = ?, next_attempt_at = ?, last_error = ? WHERE id = ?`, nextAttempts, next, errText, id); err != nil {
			return err
		}
	}
	return nil
}

func queueBackoff(attempts int) time.Duration {
	switch attempts {
	case 1:
		return 5 * time.Minute
	case 2:
		return 15 * time.Minute
	default:
		return time.Hour
	}
}

func QueueCount(ctx context.Context, db *sql.DB, userID string) int {
	if userID == "" {
		userID = DefaultUserID
	}
	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_queue WHERE user_id = ? AND processed_at IS NULL`, userID).Scan(&count)
	return count
}

func QueueDueCount(ctx context.Context, db *sql.DB, userID string) int {
	if userID == "" {
		userID = DefaultUserID
	}
	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_queue WHERE user_id = ? AND processed_at IS NULL AND attempts < ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)`, userID, maxQueueAttempts, time.Now()).Scan(&count)
	return count
}
