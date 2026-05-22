package runner

// RetryPolicy is intentionally a no-op extension point for now. The runner
// accepts it so callers can configure future LLM retry behavior without another
// request-shape migration.
func (p RetryPolicy) enabled() bool {
	return p.MaxAttempts > 1
}
