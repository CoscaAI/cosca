package cli

// Context Builder Adapter
// =============================================================================

// contextBuilderAdapter wraps context building. Placeholder.
type contextBuilderAdapter struct {
	dir string
}

// newContextBuilderAdapter creates a new context builder adapter.
func newContextBuilderAdapter(dir string) *contextBuilderAdapter {
	return &contextBuilderAdapter{dir: dir}
}

// Build builds context. Placeholder for install flow.
func (a *contextBuilderAdapter) Build() error {
	return nil
}

// =============================================================================
