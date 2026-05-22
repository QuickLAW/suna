package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultUserID       = "default"
	MaxActiveMemories   = 30
	MaxCoreMemories     = 5
	MaxInjectedMemories = 5
)

type UserMemory struct {
	ID          string
	UserID      string
	Kind        string
	Content     string
	Tags        []string
	Priority    int
	IsCore      bool
	UseCount    int
	LastUsedAt  time.Time
	RefreshedAt time.Time
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(db *sql.DB) *MemoryStore {
	return &MemoryStore{db: db}
}

func (s *MemoryStore) List(ctx context.Context, userID string, limit int) ([]UserMemory, error) {
	if userID == "" {
		userID = DefaultUserID
	}
	if limit <= 0 {
		limit = MaxActiveMemories
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, kind, content, tags, priority, is_core, use_count,
		       last_used_at, refreshed_at, expires_at, created_at, updated_at
		FROM user_memory
		WHERE user_id = ? AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY is_core DESC, priority DESC, COALESCE(last_used_at, updated_at) DESC, updated_at DESC, id ASC
		LIMIT ?`, userID, time.Now(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserMemory
	for rows.Next() {
		m, err := scanUserMemory(rows)
		if err == nil {
			out = append(out, m)
		}
	}
	return out, rows.Err()
}

func (s *MemoryStore) Count(ctx context.Context, userID string) (active, core int) {
	if userID == "" {
		userID = DefaultUserID
	}
	now := time.Now()
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_memory WHERE user_id = ? AND (expires_at IS NULL OR expires_at > ?)`, userID, now).Scan(&active)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_memory WHERE user_id = ? AND is_core = 1 AND (expires_at IS NULL OR expires_at > ?)`, userID, now).Scan(&core)
	return active, core
}

func (s *MemoryStore) BuildBrief(ctx context.Context, userID, query string) (string, []UserMemory, error) {
	mems, err := s.List(ctx, userID, MaxActiveMemories)
	if err != nil || len(mems) == 0 {
		return "", nil, err
	}
	// 召回不依赖 embedding/LLM。active memory 总量被限制在 30 条以内，注入也最多 5 条，
	// 所以选择 core + priority + 简单关键词匹配排序后的前几条。不要用硬阈值过滤普通记忆，
	// 否则中文短问句很容易因为关键词不完全匹配而漏掉已提取的稳定偏好。
	selected := selectMemories(mems, query)
	if len(selected) == 0 {
		return "", nil, nil
	}
	var lines []string
	ids := make([]string, 0, len(selected))
	for _, m := range selected {
		lines = append(lines, "- "+m.Content)
		ids = append(ids, m.ID)
	}
	_ = s.MarkUsed(ctx, ids)
	return strings.Join(lines, "\n"), selected, nil
}

func (s *MemoryStore) MarkUsed(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	now := time.Now()
	for _, id := range ids {
		_, _ = s.db.ExecContext(ctx, `UPDATE user_memory SET use_count = use_count + 1, last_used_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	}
	return nil
}

func (s *MemoryStore) ReplaceAll(ctx context.Context, userID string, newList []UserMemory) error {
	if userID == "" {
		userID = DefaultUserID
	}
	if len(newList) > MaxActiveMemories {
		newList = newList[:MaxActiveMemories]
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingRows, err := tx.QueryContext(ctx, `SELECT id, content, kind FROM user_memory WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	existing := map[string]UserMemory{}
	for existingRows.Next() {
		var m UserMemory
		if err := existingRows.Scan(&m.ID, &m.Content, &m.Kind); err == nil {
			existing[m.ID] = m
		}
	}
	existingRows.Close()

	keep := map[string]bool{}
	now := time.Now()
	for _, m := range normalizeMemoryList(userID, newList) {
		// daemon 输出的是完整的新 active memory 列表，不是 append-only patch。
		// 这里尽量复用旧 id，避免同一条记忆因为措辞不变而产生新记录。
		if m.ID == "" || existing[m.ID].ID == "" {
			m.ID = matchExistingMemory(existing, keep, m)
		}
		if m.ID == "" {
			m.ID = uuid.New().String()
		}
		keep[m.ID] = true
		tagsJSON := marshalStringSlice(m.Tags)
		isCore := 0
		if m.IsCore {
			isCore = 1
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO user_memory (id, user_id, kind, content, tags, priority, is_core, refreshed_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				kind = excluded.kind,
				content = excluded.content,
				tags = excluded.tags,
				priority = excluded.priority,
				is_core = excluded.is_core,
				refreshed_at = excluded.refreshed_at,
				updated_at = excluded.updated_at`,
			m.ID, userID, m.Kind, m.Content, tagsJSON, clampPriority(m.Priority), isCore, now, now, now)
		if err != nil {
			return err
		}
	}

	for id := range existing {
		if !keep[id] {
			// 未被 compaction 返回的旧记忆视为不再有效，直接删除，保持 active memory 小而新。
			if _, err := tx.ExecContext(ctx, `DELETE FROM user_memory WHERE id = ?`, id); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func selectMemories(mems []UserMemory, query string) []UserMemory {
	query = strings.ToLower(query)
	tokens := queryTokens(query)
	scored := make([]memoryScore, 0, len(mems))
	for _, m := range mems {
		score := m.Priority
		if m.IsCore {
			// core memory 代表长期稳定偏好/纠错，优先级远高于普通关键词命中。
			score += 1000
		}
		if len(tokens) > 0 {
			text := strings.ToLower(m.Content + " " + strings.Join(m.Tags, " ") + " " + m.Kind)
			for _, tok := range tokens {
				if strings.Contains(text, tok) {
					score += 80
				}
			}
		}
		scored = append(scored, memoryScore{Memory: m, Score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		if scored[i].Memory.IsCore != scored[j].Memory.IsCore {
			return scored[i].Memory.IsCore
		}
		if scored[i].Memory.Priority != scored[j].Memory.Priority {
			return scored[i].Memory.Priority > scored[j].Memory.Priority
		}
		return scored[i].Memory.ID < scored[j].Memory.ID
	})
	limit := MaxInjectedMemories
	if len(scored) < limit {
		limit = len(scored)
	}
	out := make([]UserMemory, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, scored[i].Memory)
	}
	return out
}

type memoryScore struct {
	Memory UserMemory
	Score  int
}

func queryTokens(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' || r == '。' || r == '，' || r == ':' || r == '：' || r == ';' || r == '；' || r == '?' || r == '？' || r == '!' || r == '！'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if len([]rune(f)) >= 2 {
			out = append(out, f)
		}
	}
	return out
}

func normalizeMemoryList(userID string, in []UserMemory) []UserMemory {
	out := make([]UserMemory, 0, len(in))
	core := 0
	seen := map[string]bool{}
	for _, m := range in {
		m.UserID = userID
		m.Kind = normalizeKind(m.Kind)
		m.Content = strings.TrimSpace(m.Content)
		if m.Content == "" || seen[strings.ToLower(m.Content)] {
			continue
		}
		seen[strings.ToLower(m.Content)] = true
		if len([]rune(m.Content)) > 180 {
			m.Content = truncateRunes(m.Content, 180)
		}
		if m.IsCore {
			core++
			if core > MaxCoreMemories {
				m.IsCore = false
			}
		}
		out = append(out, m)
		if len(out) >= MaxActiveMemories {
			break
		}
	}
	return out
}

func normalizeKind(kind string) string {
	switch strings.TrimSpace(strings.ToLower(kind)) {
	case "preference", "habit", "constraint", "correction", "personality", "fact":
		return strings.TrimSpace(strings.ToLower(kind))
	default:
		return "preference"
	}
}

func matchExistingMemory(existing map[string]UserMemory, keep map[string]bool, m UserMemory) string {
	for id, old := range existing {
		if keep[id] {
			continue
		}
		if old.Kind == m.Kind && strings.EqualFold(strings.TrimSpace(old.Content), strings.TrimSpace(m.Content)) {
			return id
		}
	}
	return ""
}

func scanUserMemory(rows interface{ Scan(dest ...any) error }) (UserMemory, error) {
	var m UserMemory
	var tags string
	var isCore int
	var lastUsed, refreshed, expires, created, updated sql.NullString
	err := rows.Scan(&m.ID, &m.UserID, &m.Kind, &m.Content, &tags, &m.Priority, &isCore, &m.UseCount, &lastUsed, &refreshed, &expires, &created, &updated)
	if err != nil {
		return m, err
	}
	m.Tags = unmarshalStringSlice(tags)
	m.IsCore = isCore == 1
	m.LastUsedAt = parseDBTime(lastUsed.String)
	m.RefreshedAt = parseDBTime(refreshed.String)
	m.ExpiresAt = parseDBTime(expires.String)
	m.CreatedAt = parseDBTime(created.String)
	m.UpdatedAt = parseDBTime(updated.String)
	return m, nil
}

func marshalStringSlice(v []string) string {
	if v == nil {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalStringSlice(s string) []string {
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

func parseDBTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func clampPriority(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func (m UserMemory) String() string {
	return fmt.Sprintf("%s: %s", m.Kind, m.Content)
}
