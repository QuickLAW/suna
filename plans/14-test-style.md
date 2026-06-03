# 14 — Go Test Style

Suna tests follow idiomatic Go testing practices. Keep tests simple, deterministic, and close to the code they verify.

## Defaults

- Use the standard `testing` package by default.
- Do not introduce assertion frameworks such as `testify` for ordinary tests.
- Keep tests next to the package under test as `*_test.go` files.
- Prefer same-package tests (`package foo`) for Suna `internal` packages, because most tests are white-box tests of internal behavior.
- Use `package foo_test` only when intentionally testing the exported API from an external user's perspective.

## Naming

Name tests by behavior:

```go
func TestSubjectBehavior(t *testing.T) {}
```

Examples:

```go
func TestGuardRejectsWorkspaceEscape(t *testing.T) {}
func TestSkillRuntimeRequiresExplicitEnableAfterImport(t *testing.T) {}
func TestConfigSaveOmitsDefaultMaxModelRPS(t *testing.T) {}
```

Avoid vague names and avoid binding tests to implementation details unless the implementation detail is the unit being tested.

## Table-driven tests

Use table-driven tests for repeated cases of the same behavior.

```go
func TestSubjectBehavior(t *testing.T) {
	tests := []struct {
		name string
		input string
		want string
	}{
		{name: "simple case", input: "in", want: "out"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := subject(tt.input)
			if got != tt.want {
				t.Fatalf("subject(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

Guidelines:

- Put `name` first.
- Use short, lower-case, human-readable case names.
- Use `t.Run` for every case.
- Keep `tt := tt` before the closure for explicit capture.

## Assertions

Use `got` and `want` in failure messages.

Preferred:

```go
got := cfg.GetMaxModelRPS()
want := DefaultMaxModelRPS
if got != want {
	t.Fatalf("GetMaxModelRPS() = %d, want %d", got, want)
}
```

For expected errors:

```go
if err == nil {
	t.Fatalf("NormalizeGuard() error = nil, want non-nil")
}
```

For unexpected errors:

```go
if err != nil {
	t.Fatalf("Save() error = %v", err)
}
```

Prefer fail-fast `t.Fatalf`/`t.Fatal` unless a test intentionally aggregates independent failures.

## Helpers and fixtures

Test helpers must call `t.Helper()`.

```go
func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
```

Prefer package-local helpers over a global test utility package.

## Isolation

- Use `t.TempDir()` for filesystem tests.
- Use `t.Setenv()` for environment changes.
- Unit tests must not access the real network, call real LLMs, depend on `~/.suna`, or depend on user-local configuration.
- Use fakes for reviewers, prompters, model providers, transports, and external services.

## Integration tests

Tests that require a daemon, network, real model provider, or other external dependency must be opt-in with a build tag:

```go
//go:build integration
```

Run them explicitly:

```bash
go test -tags=integration ./...
```

Plain `go test ./...` must remain fast, offline, and deterministic.

## TUI tests

TUI tests should prefer state and behavior checks over brittle full-screen output comparisons.

- Keep rendering tests as focused smoke tests for important semantic text.
- Do not compare full terminal screens unless there is a strong reason.
- Strip ANSI when checking rendered text.
- Avoid coupling tests to incidental layout details.
