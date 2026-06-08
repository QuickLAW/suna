package mcp

import (
	"context"
	"testing"

	"github.com/alanchenchen/suna/internal/config"
)

func TestSetActiveFalseUpdatesConfigSnapshot(t *testing.T) {
	r := NewRuntime(config.MCPConfig{Servers: map[string]config.MCPServerConfig{
		"github": {Enabled: true, Transport: TransportStdio, Command: "npx", Args: []string{"server"}},
	}})

	if err := r.SetActive(context.Background(), "github", false); err != nil {
		t.Fatalf("SetActive() error = %v", err)
	}
	cfg := r.Config()
	if got := cfg.Servers["github"].Enabled; got {
		t.Fatalf("Config().Servers[github].Enabled = %t, want false", got)
	}

	cfg.Servers["github"] = config.MCPServerConfig{Enabled: true}
	if got := r.Config().Servers["github"].Enabled; got {
		t.Fatalf("Config() returned mutable snapshot, enabled = %t, want false", got)
	}
}
