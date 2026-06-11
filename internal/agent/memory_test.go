package agent

import (
	"context"
	"testing"

	"github.com/alanchenchen/suna/internal/memory"
	"github.com/alanchenchen/suna/internal/model"
)

func TestEnqueueMemoryEventIgnoresAssistantOutput(t *testing.T) {
	store, err := memory.NewStore(t.TempDir() + "/memory.db")
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	agent := &Agent{extractQueue: memory.NewExtractQueue(store.DB())}
	agent.enqueueMemoryEvent(context.Background(), model.RoleAssistant, "以后都按这个 TUI 方案实现", false, false, false, false)

	if got := memory.QueueCount(context.Background(), store.DB(), memory.DefaultUserID); got != 0 {
		t.Fatalf("QueueCount() = %d, want 0", got)
	}
}

func TestEnqueueMemoryEventQueuesUserProfileCandidate(t *testing.T) {
	store, err := memory.NewStore(t.TempDir() + "/memory.db")
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	agent := &Agent{extractQueue: memory.NewExtractQueue(store.DB())}
	agent.enqueueMemoryEvent(context.Background(), model.RoleUser, "以后回复先给我简短结论", false, false, false, false)

	if got := memory.QueueCount(context.Background(), store.DB(), memory.DefaultUserID); got != 1 {
		t.Fatalf("QueueCount() = %d, want 1", got)
	}
}
